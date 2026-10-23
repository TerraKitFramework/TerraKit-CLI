#ifndef TK_GO_BRIDGE_H
#define TK_GO_BRIDGE_H
#include <stdint.h>
#include <stddef.h>

typedef struct { const uint8_t* data; size_t length; } tk_string_view;
typedef struct { uint32_t major,minor,patch; } tk_version;
typedef struct { double x,y; } tk_dvec2;
typedef struct { double x,y,z; } tk_dvec3;
typedef struct { float x,y,z; } tk_fvec3;
typedef struct { float x,y; } tk_fvec2;
typedef struct {
    int32_t kind; uint8_t boolean, reserved_bool[3];
    int64_t signed_value; uint64_t unsigned_value;
    float float32_value; uint32_t reserved_float32;
    double float64_value; tk_dvec2 vec2; tk_dvec3 vec3; tk_string_view text;
} tk_param_value;
typedef struct {
    tk_string_view type_id; uint32_t version;
    tk_string_view display_name,category,description;
    size_t inputs,outputs,parameters;
} tk_schema;
typedef struct {
    tk_string_view id,display_name,description;
    int32_t resource_kind; uint8_t optional;
} tk_input;
typedef struct {
    tk_string_view id,display_name,description; int32_t resource_kind;
} tk_output;
typedef struct {
    tk_string_view id,display_name,description;
    int32_t kind; uint8_t has_default; tk_param_value default_value;
    uint8_t has_min; tk_param_value min_value;
    uint8_t has_max; tk_param_value max_value;
    size_t enum_options;
} tk_parameter;
typedef struct { tk_string_view id,display_name,description; } tk_enum_option;
typedef struct { uint32_t cell_width,cell_height; tk_dvec2 spacing; } tk_layout2;
typedef struct { uint64_t seed; int64_t x,y; uint16_t lod; uint8_t reserved[6]; } tk_request2;
typedef struct {
    tk_dvec3 origin;
    const tk_fvec3 *positions; size_t position_count;
    const uint32_t *indices; size_t index_count;
    const tk_fvec3 *normals; size_t normal_count;
    const tk_fvec2 *uvs; size_t uv_count;
} tk_mesh_view;

void *go_tk_open(const char *path);
void go_tk_close(void *library);
const char *go_tk_loader_error(void);
int go_tk_error(void *lib, char *buf, size_t capacity, size_t *required);
int go_tk_abi(void *lib, tk_version *out);
int go_tk_library_version(void *lib, tk_string_view *out);
int go_tk_registry_create(void *lib, void **out);
int go_tk_registry_destroy(void *lib, void **handle);
int go_tk_schema_count(void *lib, void *registry, size_t *out);
int go_tk_schema(void *lib, void *registry, size_t index, tk_schema *out);
int go_tk_input(void *lib, void *registry, size_t schema, size_t index, tk_input *out);
int go_tk_output(void *lib, void *registry, size_t schema, size_t index, tk_output *out);
int go_tk_parameter(void *lib, void *registry, size_t schema, size_t index, tk_parameter *out);
int go_tk_enum_option(void *lib, void *registry, size_t schema, size_t parameter, size_t index, tk_enum_option *out);
int go_tk_stage_create(void *lib, uint64_t id, tk_string_view type_id, uint32_t version, void **out);
int go_tk_stage_destroy(void *lib, void **stage);
int go_tk_stage_parameter(void *lib, void *stage, tk_string_view id, const tk_param_value *value);
int go_tk_stage_input(void *lib, void *stage, tk_string_view id, uint64_t key);
int go_tk_stage_output(void *lib, void *stage, tk_string_view id, uint64_t key);
int go_tk_assembler_create(void *lib, void *registry, void **out);
int go_tk_assembler_destroy(void *lib, void **assembler);
int go_tk_assembler_add(void *lib, void *assembler, void **stage);
int go_tk_assembler_finish(void *lib, void **assembler, void **pipeline);
int go_tk_pipeline_destroy(void *lib, void **pipeline);
int go_tk_runtime_create(void *lib, void **pipeline, const tk_layout2 *layout, void **out);
int go_tk_runtime_generate(void *lib, void *runtime, const tk_request2 *req, void **out);
int go_tk_runtime_destroy(void *lib, void **runtime);
int go_tk_result_mesh(void *lib, void *result, uint64_t key, tk_mesh_view *out);
int go_tk_result_destroy(void *lib, void **result);
#endif
