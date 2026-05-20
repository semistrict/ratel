load("@go_googleapis//:repository_rules.bzl", "switched_rules_by_language")

def _googleapis_ext_impl(_module_ctx):
    switched_rules_by_language(
        name = "com_google_googleapis_imports",
        go = True,
        grpc = True,
    )

googleapis_ext = module_extension(
    implementation = _googleapis_ext_impl,
)
