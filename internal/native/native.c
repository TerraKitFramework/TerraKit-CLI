#include "native.h"
#include <stdio.h>
#include <string.h>
#if defined(_WIN32)
#include <windows.h>
#define OPEN(p) ((void*)LoadLibraryA(p))
#define SYM(h,s) ((void*)GetProcAddress((HMODULE)(h),(s)))
#define CLOSE(h) FreeLibrary((HMODULE)(h))
#elif defined(__APPLE__)
#include <dlfcn.h>
#define OPEN(p) dlopen((p), RTLD_NOW | RTLD_LOCAL)
#define SYM(h,s) dlsym((h),(s))
#define CLOSE(h) dlclose((h))
#else
#include <dlfcn.h>
#define OPEN(p) dlopen((p), RTLD_NOW | RTLD_LOCAL)
#define SYM(h,s) dlsym((h),(s))
#define CLOSE(h) dlclose((h))
#endif

_Static_assert(sizeof(void*)==8, "TerraKit requires a 64-bit process");
_Static_assert(sizeof(tk_param_value)==96, "TerraKit parameter ABI layout mismatch");
_Static_assert(sizeof(tk_schema)==96, "TerraKit schema ABI layout mismatch");
_Static_assert(sizeof(tk_input)==56, "TerraKit input ABI layout mismatch");
_Static_assert(sizeof(tk_output)==56, "TerraKit output ABI layout mismatch");
_Static_assert(sizeof(tk_parameter)==368, "TerraKit parameter schema layout mismatch");
_Static_assert(sizeof(tk_enum_option)==48, "TerraKit enum layout mismatch");
_Static_assert(sizeof(tk_mesh_view)==88, "TerraKit mesh ABI layout mismatch");
_Static_assert(sizeof(tk_layout2)==24, "TerraKit layout ABI mismatch");
_Static_assert(sizeof(tk_request2)==32, "TerraKit request ABI mismatch");

static char loader_error[512];
void *go_tk_open(const char *path) {
    void *lib = OPEN(path);
    if (!lib) {
#if defined(_WIN32)
        snprintf(loader_error, sizeof(loader_error), "LoadLibrary failed for %s (Windows error %lu)", path, (unsigned long)GetLastError());
#else
        const char *err = dlerror();
        snprintf(loader_error, sizeof(loader_error), "%s", err ? err : "dlopen failed");
#endif
    }
    return lib;
}
void go_tk_close(void *lib) { if(lib) CLOSE(lib); }
const char *go_tk_loader_error(void) { return loader_error; }
#define RESOLVE(name, sig) \
    typedef int (*fn_t) sig; \
    fn_t fn = (fn_t) SYM(lib, name); \
    if (!fn) { snprintf(loader_error, sizeof(loader_error), "missing TerraKit symbol: %s", name); return -1; }
int go_tk_error(void *lib,char *buf,size_t cap,size_t *required){ RESOLVE("tk_last_error_message_copy",(char*,size_t,size_t*));return fn(buf,cap,required); }
int go_tk_abi(void *lib,tk_version *out){RESOLVE("tk_get_abi_version",(tk_version*));return fn(out);}
int go_tk_library_version(void *lib,tk_string_view *out){RESOLVE("tk_get_library_version",(tk_string_view*));return fn(out);}
int go_tk_registry_create(void *lib,void **out){RESOLVE("tk_stage_registry_create_builtin",(void**));return fn(out);}
int go_tk_registry_destroy(void *lib,void **handle){RESOLVE("tk_stage_registry_destroy",(void**));return fn(handle);}
int go_tk_schema_count(void *lib,void *registry,size_t *out){RESOLVE("tk_stage_registry_get_schema_count",(void*,size_t*));return fn(registry,out);}
int go_tk_schema(void *lib,void *registry,size_t index,tk_schema *out){RESOLVE("tk_stage_registry_get_schema",(void*,size_t,tk_schema*));return fn(registry,index,out);}
int go_tk_input(void *lib,void *registry,size_t schema,size_t index,tk_input *out){RESOLVE("tk_stage_registry_get_input_port",(void*,size_t,size_t,tk_input*));return fn(registry,schema,index,out);}
int go_tk_output(void *lib,void *registry,size_t schema,size_t index,tk_output *out){RESOLVE("tk_stage_registry_get_output_port",(void*,size_t,size_t,tk_output*));return fn(registry,schema,index,out);}
int go_tk_parameter(void *lib,void *registry,size_t schema,size_t index,tk_parameter *out){RESOLVE("tk_stage_registry_get_parameter",(void*,size_t,size_t,tk_parameter*));return fn(registry,schema,index,out);}
int go_tk_enum_option(void *lib,void *registry,size_t schema,size_t parameter,size_t index,tk_enum_option *out){RESOLVE("tk_stage_registry_get_enum_option",(void*,size_t,size_t,size_t,tk_enum_option*));return fn(registry,schema,parameter,index,out);}
int go_tk_stage_create(void *lib,uint64_t id,tk_string_view type_id,uint32_t version,void **out){RESOLVE("tk_stage_construction_create",(uint64_t,tk_string_view,uint32_t,void**));return fn(id,type_id,version,out);}
int go_tk_stage_destroy(void *lib,void **stage){RESOLVE("tk_stage_construction_destroy",(void**));return fn(stage);}
int go_tk_stage_parameter(void *lib,void *stage,tk_string_view id,const tk_param_value *value){RESOLVE("tk_stage_construction_set_parameter",(void*,tk_string_view,const tk_param_value*));return fn(stage,id,value);}
int go_tk_stage_input(void *lib,void *stage,tk_string_view id,uint64_t key){RESOLVE("tk_stage_construction_bind_input",(void*,tk_string_view,uint64_t));return fn(stage,id,key);}
int go_tk_stage_output(void *lib,void *stage,tk_string_view id,uint64_t key){RESOLVE("tk_stage_construction_bind_output",(void*,tk_string_view,uint64_t));return fn(stage,id,key);}
int go_tk_assembler_create(void *lib,void *registry,void **out){RESOLVE("tk_pipeline_assembler_create",(void*,void**));return fn(registry,out);}
int go_tk_assembler_destroy(void *lib,void **assembler){RESOLVE("tk_pipeline_assembler_destroy",(void**));return fn(assembler);}
int go_tk_assembler_add(void *lib,void *assembler,void **stage){RESOLVE("tk_pipeline_assembler_add_stage",(void*,void**));return fn(assembler,stage);}
int go_tk_assembler_finish(void *lib,void **assembler,void **pipeline){RESOLVE("tk_pipeline_assembler_finish",(void**,void**));return fn(assembler,pipeline);}
int go_tk_pipeline_destroy(void *lib,void **pipeline){RESOLVE("tk_pipeline_destroy",(void**));return fn(pipeline);}
int go_tk_runtime_create(void *lib,void **pipeline,const tk_layout2 *layout,void **out){RESOLVE("tk_runtime_create_2d",(void**,const tk_layout2*,void**));return fn(pipeline,layout,out);}
int go_tk_runtime_generate(void *lib,void *runtime,const tk_request2 *req,void **out){RESOLVE("tk_runtime_generate_2d",(void*,const tk_request2*,void**));return fn(runtime,req,out);}
int go_tk_runtime_destroy(void *lib,void **runtime){RESOLVE("tk_runtime_destroy",(void**));return fn(runtime);}
int go_tk_result_mesh(void *lib,void *result,uint64_t key,tk_mesh_view *out){RESOLVE("tk_generation_result_get_mesh",(void*,uint64_t,tk_mesh_view*));return fn(result,key,out);}
int go_tk_result_destroy(void *lib,void **result){RESOLVE("tk_generation_result_destroy",(void**));return fn(result);}
