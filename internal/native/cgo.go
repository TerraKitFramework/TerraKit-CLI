//go:build cgo

package native

/*
#cgo linux LDFLAGS: -ldl
#include "native.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/mesh"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/validate"
)

type Library struct {
	ptr     unsafe.Pointer
	version string
}

func Open(path string) (*Library, error) {
	executable, _ := os.Executable()
	path = resolveLibraryPath(path, executable, runtime.GOOS)
	raw := C.CString(path)
	defer C.free(unsafe.Pointer(raw))
	lib := &Library{ptr: C.go_tk_open(raw)}
	if lib.ptr == nil {
		return nil, fmt.Errorf("load TerraKit %q: %s", path, C.GoString(C.go_tk_loader_error()))
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var abi C.tk_version
	if err := lib.check(C.go_tk_abi(lib.ptr, &abi), "get ABI"); err != nil {
		lib.Close()
		return nil, err
	}
	if abi.major != 0 || abi.minor != 2 {
		lib.Close()
		return nil, fmt.Errorf("unsupported TerraKit ABI %d.%d.%d (expected 0.2.x)", abi.major, abi.minor, abi.patch)
	}
	var v C.tk_string_view
	if err := lib.check(C.go_tk_library_version(lib.ptr, &v), "get library version"); err != nil {
		lib.Close()
		return nil, err
	}
	lib.version = strview(v)
	return lib, nil
}
func (l *Library) Version() string { return l.version }
func (l *Library) Close() {
	if l.ptr != nil {
		C.go_tk_close(l.ptr)
		l.ptr = nil
	}
}
func (l *Library) check(status C.int, op string) error {
	if status == 0 {
		return nil
	}
	if status == -1 {
		return fmt.Errorf("%s: %s", op, C.GoString(C.go_tk_loader_error()))
	}
	var needed C.size_t
	C.go_tk_error(l.ptr, nil, 0, &needed)
	msg := ""
	if needed > 0 && needed < 65536 {
		buf := make([]byte, int(needed))
		C.go_tk_error(l.ptr, (*C.char)(unsafe.Pointer(&buf[0])), needed, &needed)
		msg = strings.TrimRight(string(buf), "\x00")
	}
	return fmt.Errorf("%s: TerraKit status %d: %s", op, int(status), msg)
}
func strview(v C.tk_string_view) string {
	if v.data == nil || v.length == 0 {
		return ""
	}
	return C.GoStringN((*C.char)(unsafe.Pointer(v.data)), C.int(v.length))
}
func asview(s string) (C.tk_string_view, func()) {
	if len(s) == 0 {
		return C.tk_string_view{}, func() {}
	}
	p := C.CBytes([]byte(s))
	return C.tk_string_view{data: (*C.uint8_t)(p), length: C.size_t(len(s))}, func() { C.free(p) }
}
func (l *Library) Catalog() ([]catalog.Stage, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var registry unsafe.Pointer
	if err := l.check(C.go_tk_registry_create(l.ptr, &registry), "create stage registry"); err != nil {
		return nil, err
	}
	defer C.go_tk_registry_destroy(l.ptr, &registry)
	var count C.size_t
	if err := l.check(C.go_tk_schema_count(l.ptr, registry, &count), "schema count"); err != nil {
		return nil, err
	}
	if count > 10000 {
		return nil, fmt.Errorf("too many stage schemas: %d", count)
	}
	stages := make([]catalog.Stage, 0, int(count))
	for i := 0; i < int(count); i++ {
		var s C.tk_schema
		if err := l.check(C.go_tk_schema(l.ptr, registry, C.size_t(i), &s), "read schema"); err != nil {
			return nil, err
		}
		stage := catalog.Stage{TypeID: strview(s.type_id), Version: uint32(s.version), Name: strview(s.display_name), Category: strview(s.category), Description: strview(s.description)}
		if s.inputs > 1000 || s.outputs > 1000 || s.parameters > 1000 {
			return nil, fmt.Errorf("implausible schema size: %s", stage.TypeID)
		}
		for j := 0; j < int(s.inputs); j++ {
			var p C.tk_input
			if err := l.check(C.go_tk_input(l.ptr, registry, C.size_t(i), C.size_t(j), &p), "read input"); err != nil {
				return nil, err
			}
			stage.Inputs = append(stage.Inputs, catalog.Port{ID: strview(p.id), Name: strview(p.display_name), Kind: int(p.resource_kind), Optional: p.optional != 0})
		}
		for j := 0; j < int(s.outputs); j++ {
			var p C.tk_output
			if err := l.check(C.go_tk_output(l.ptr, registry, C.size_t(i), C.size_t(j), &p), "read output"); err != nil {
				return nil, err
			}
			stage.Outputs = append(stage.Outputs, catalog.Port{ID: strview(p.id), Name: strview(p.display_name), Kind: int(p.resource_kind)})
		}
		for j := 0; j < int(s.parameters); j++ {
			var p C.tk_parameter
			if err := l.check(C.go_tk_parameter(l.ptr, registry, C.size_t(i), C.size_t(j), &p), "read parameter"); err != nil {
				return nil, err
			}
			param := catalog.Parameter{ID: strview(p.id), Name: strview(p.display_name), Kind: int(p.kind), Description: strview(p.description)}
			if p.has_default != 0 {
				param.Default = paramText(p.default_value)
			}
			if p.has_min != 0 {
				v := paramText(p.min_value)
				param.Min = &v
			}
			if p.has_max != 0 {
				v := paramText(p.max_value)
				param.Max = &v
			}
			if p.enum_options > 10000 {
				return nil, fmt.Errorf("too many enum options")
			}
			for k := 0; k < int(p.enum_options); k++ {
				var o C.tk_enum_option
				if err := l.check(C.go_tk_enum_option(l.ptr, registry, C.size_t(i), C.size_t(j), C.size_t(k), &o), "read enum option"); err != nil {
					return nil, err
				}
				param.Options = append(param.Options, strview(o.id))
			}
			stage.Parameters = append(stage.Parameters, param)
		}
		stages = append(stages, stage)
	}
	return stages, nil
}
func paramText(v C.tk_param_value) string {
	switch int(v.kind) {
	case 1:
		if v.boolean != 0 {
			return "true"
		}
		return "false"
	case 2:
		return strconv.FormatInt(int64(v.signed_value), 10)
	case 3:
		return strconv.FormatUint(uint64(v.unsigned_value), 10)
	case 4:
		return strconv.FormatFloat(float64(v.float32_value), 'g', -1, 32)
	case 5:
		return strconv.FormatFloat(float64(v.float64_value), 'g', -1, 64)
	case 6:
		return fmt.Sprintf("%g,%g", v.vec2.x, v.vec2.y)
	case 7:
		return fmt.Sprintf("%g,%g,%g", v.vec3.x, v.vec3.y, v.vec3.z)
	case 8, 9:
		return strview(v.text)
	default:
		return ""
	}
}
func parameterValue(p catalog.Parameter, text string) (C.tk_param_value, func(), error) {
	v := C.tk_param_value{kind: C.int32_t(p.Kind)}
	noop := func() {}
	switch p.Kind {
	case 1:
		b, err := strconv.ParseBool(text)
		if err != nil {
			return v, noop, err
		}
		if b {
			v.boolean = 1
		}
	case 2:
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return v, noop, err
		}
		v.signed_value = C.int64_t(n)
	case 3:
		n, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return v, noop, err
		}
		v.unsigned_value = C.uint64_t(n)
	case 4, 5:
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return v, noop, err
		}
		if p.Kind == 4 {
			v.float32_value = C.float(n)
		} else {
			v.float64_value = C.double(n)
		}
	case 6, 7:
		parts := strings.Split(strings.Trim(text, " ()"), ",")
		expected := 2
		if p.Kind == 7 {
			expected = 3
		}
		if len(parts) != expected {
			return v, noop, fmt.Errorf("expected %d vector components", expected)
		}
		vals := make([]float64, expected)
		for i, s := range parts {
			n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				return v, noop, err
			}
			vals[i] = n
		}
		if p.Kind == 6 {
			v.vec2 = C.tk_dvec2{x: C.double(vals[0]), y: C.double(vals[1])}
		} else {
			v.vec3 = C.tk_dvec3{x: C.double(vals[0]), y: C.double(vals[1]), z: C.double(vals[2])}
		}
	case 8, 9:
		var free func()
		v.text, free = asview(text)
		return v, free, nil
	default:
		return v, noop, fmt.Errorf("unsupported parameter kind %d", p.Kind)
	}
	return v, noop, nil
}
func (l *Library) Generate(cfg config.Document, stages []catalog.Stage) (mesh.Data, error) {
	ordered, err := validate.Order(cfg, stages)
	if err != nil {
		return mesh.Data{}, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var registry, assembler, construction, pipeline, rt, result unsafe.Pointer
	// A pipeline owns each stage; runtime creation consumes the pipeline.
	defer func() {
		if result != nil {
			C.go_tk_result_destroy(l.ptr, &result)
		}
		if rt != nil {
			C.go_tk_runtime_destroy(l.ptr, &rt)
		}
		if pipeline != nil {
			C.go_tk_pipeline_destroy(l.ptr, &pipeline)
		}
		if construction != nil {
			C.go_tk_stage_destroy(l.ptr, &construction)
		}
		if assembler != nil {
			C.go_tk_assembler_destroy(l.ptr, &assembler)
		}
		if registry != nil {
			C.go_tk_registry_destroy(l.ptr, &registry)
		}
	}()
	if err := l.check(C.go_tk_registry_create(l.ptr, &registry), "create registry"); err != nil {
		return mesh.Data{}, err
	}
	if err := l.check(C.go_tk_assembler_create(l.ptr, registry, &assembler), "create assembler"); err != nil {
		return mesh.Data{}, err
	}
	byType := catalog.ByID(stages)
	keys := map[string]uint64{}
	var nextKey uint64 = 1
	var selected uint64
	for i, n := range ordered {
		schema := byType[n.Type]
		typeid, free := asview(n.Type)
		status := C.go_tk_stage_create(l.ptr, C.uint64_t(i+1), typeid, C.uint32_t(schema.Version), &construction)
		free()
		if err := l.check(status, "create stage "+n.ID); err != nil {
			return mesh.Data{}, err
		}
		for _, p := range schema.Inputs {
			for _, link := range cfg.Links {
				if link.ToNode == n.ID && link.ToPort == p.ID {
					key := keys[link.FromNode+"/"+link.FromPort]
					id, done := asview(p.ID)
					status = C.go_tk_stage_input(l.ptr, construction, id, C.uint64_t(key))
					done()
					if err := l.check(status, "bind input "+n.ID+"."+p.ID); err != nil {
						return mesh.Data{}, err
					}
				}
			}
		}
		for _, p := range schema.Outputs {
			k := nextKey
			nextKey++
			keys[n.ID+"/"+p.ID] = k
			if cfg.Output.Node == n.ID && cfg.Output.Port == p.ID {
				selected = k
			}
			id, done := asview(p.ID)
			status := C.go_tk_stage_output(l.ptr, construction, id, C.uint64_t(k))
			done()
			if err := l.check(status, "bind output "+n.ID+"."+p.ID); err != nil {
				return mesh.Data{}, err
			}
		}
		for _, p := range schema.Parameters {
			val := p.Default
			if custom, ok := n.Parameters[p.ID]; ok {
				val = custom
			}
			nativeValue, cleanup, err := parameterValue(p, val)
			if err != nil {
				return mesh.Data{}, fmt.Errorf("%s.%s: %w", n.ID, p.ID, err)
			}
			id, done := asview(p.ID)
			status := C.go_tk_stage_parameter(l.ptr, construction, id, &nativeValue)
			done()
			cleanup()
			if err := l.check(status, "set parameter "+n.ID+"."+p.ID); err != nil {
				return mesh.Data{}, err
			}
		}
		if err := l.check(C.go_tk_assembler_add(l.ptr, assembler, &construction), "add stage "+n.ID); err != nil {
			return mesh.Data{}, err
		}
	}
	if selected == 0 {
		return mesh.Data{}, fmt.Errorf("mesh output not selected")
	}
	if err := l.check(C.go_tk_assembler_finish(l.ptr, &assembler, &pipeline), "finish pipeline"); err != nil {
		return mesh.Data{}, err
	}
	layout := C.tk_layout2{cell_width: C.uint32_t(cfg.Region.Width), cell_height: C.uint32_t(cfg.Region.Height), spacing: C.tk_dvec2{x: C.double(cfg.Region.SpacingX), y: C.double(cfg.Region.SpacingY)}}
	if err := l.check(C.go_tk_runtime_create(l.ptr, &pipeline, &layout, &rt), "create runtime"); err != nil {
		return mesh.Data{}, err
	}
	request := C.tk_request2{seed: C.uint64_t(cfg.Region.Seed), x: C.int64_t(cfg.Region.X), y: C.int64_t(cfg.Region.Y), lod: C.uint16_t(cfg.Region.LOD)}
	if err := l.check(C.go_tk_runtime_generate(l.ptr, rt, &request, &result), "generate region"); err != nil {
		return mesh.Data{}, err
	}
	var view C.tk_mesh_view
	if err := l.check(C.go_tk_result_mesh(l.ptr, result, C.uint64_t(selected), &view), "fetch mesh"); err != nil {
		return mesh.Data{}, err
	}
	if view.position_count > 5_000_000 || view.index_count > 30_000_000 {
		return mesh.Data{}, fmt.Errorf("native mesh exceeds safety limit")
	}
	if view.position_count == 0 || view.index_count == 0 || view.positions == nil || view.indices == nil {
		return mesh.Data{}, fmt.Errorf("native mesh has no geometry")
	}
	m := mesh.Data{Origin: mesh.Vec3{float64(view.origin.x), float64(view.origin.y), float64(view.origin.z)}}
	for _, v := range unsafe.Slice(view.positions, int(view.position_count)) {
		m.Positions = append(m.Positions, mesh.Vec3{float64(v.x), float64(v.y), float64(v.z)})
	}
	for _, v := range unsafe.Slice(view.indices, int(view.index_count)) {
		m.Indices = append(m.Indices, uint32(v))
	}
	if view.normal_count > 0 {
		if view.normals == nil || view.normal_count != view.position_count {
			return mesh.Data{}, fmt.Errorf("bad native normal buffer")
		}
		for _, v := range unsafe.Slice(view.normals, int(view.normal_count)) {
			m.Normals = append(m.Normals, mesh.Vec3{float64(v.x), float64(v.y), float64(v.z)})
		}
	}
	if view.uv_count > 0 {
		if view.uvs == nil || view.uv_count != view.position_count {
			return mesh.Data{}, fmt.Errorf("bad native uv buffer")
		}
		for _, v := range unsafe.Slice(view.uvs, int(view.uv_count)) {
			m.UVs = append(m.UVs, mesh.Vec2{float64(v.x), float64(v.y)})
		}
	}
	return m, nil
}
