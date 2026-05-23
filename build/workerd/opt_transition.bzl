def _opt_transition_impl(settings, attr):
    return {"//command_line_option:compilation_mode": "opt"}

_opt_transition = transition(
    implementation = _opt_transition_impl,
    inputs = [],
    outputs = ["//command_line_option:compilation_mode"],
)

def _opt_binary_impl(ctx):
    executable = ctx.executable.binary
    if executable == None:
        fail("%s does not provide an executable" % ctx.attr.binary.label)

    out = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.run_shell(
        inputs = [executable],
        outputs = [out],
        command = "cp \"$1\" \"$2\" && chmod 0755 \"$2\"",
        arguments = [executable.path, out.path],
    )
    return [DefaultInfo(
        executable = out,
        files = depset([out]),
        runfiles = ctx.runfiles(files = [out]),
    )]

opt_binary = rule(
    implementation = _opt_binary_impl,
    attrs = {
        "binary": attr.label(
            cfg = _opt_transition,
            executable = True,
            mandatory = True,
        ),
        "_allowlist_function_transition": attr.label(
            default = "@bazel_tools//tools/allowlists/function_transition_allowlist",
        ),
    },
    executable = True,
)
