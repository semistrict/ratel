# Define the top level namespace. This lets everything be addressable using
# `@com_github_cockroachdb_cockroach//...`.
workspace(name = "com_github_cockroachdb_cockroach")

# Load the things that let us load other things.
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Load go bazel tools. This gives us access to the go bazel SDK/toolchains.
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "86d3dc8f59d253524f933aaf2f3c05896cb0b605fc35b460c0b4b039996124c6",
    urls = [
        "https://github.com/bazel-contrib/rules_go/releases/download/v0.60.0/rules_go-v0.60.0.zip",
    ],
)

# Like the above, but for JS.
http_archive(
    name = "aspect_rules_js",
    sha256 = "4cd58cc27167ac7319de2039f936c711055a823f69dd457409226f5ec8e4237e",
    strip_prefix = "rules_js-3.1.2",
    url = "https://github.com/aspect-build/rules_js/releases/download/v3.1.2/rules_js-v3.1.2.tar.gz",
)

http_archive(
    name = "aspect_rules_js_npm_translate",
    patch_args = ["-p1"],
    patches = ["//build/patches:aspect_rules_js_npm_translate_repo_name.patch"],
    sha256 = "08061ba5e5e7f4b1074538323576dac819f9337a0c7d75aee43afc8ae7cb6e18",
    strip_prefix = "rules_js-1.26.1",
    url = "https://storage.googleapis.com/public-bazel-artifacts/js/rules_js-v1.26.1.tar.gz",
)

http_archive(
    name = "aspect_bazel_lib",
    patch_args = ["-p1"],
    patches = ["//build/patches:aspect_bazel_lib.patch"],
    sha256 = "0da75299c5a52737b2ac39458398b3f256e41a1a6748e5457ceb3a6225269485",
    strip_prefix = "bazel-lib-1.31.2",
    url = "https://storage.googleapis.com/public-bazel-artifacts/bazel/bazel-lib-v1.31.2.tar.gz",
)

http_archive(
    name = "aspect_rules_ts",
    sha256 = "06a432998e3f0b4c1057926b3946b51057413c6ffcb19bbc5e2674191a061063",
    strip_prefix = "rules_ts-3.8.10",
    url = "https://github.com/aspect-build/rules_ts/releases/download/v3.8.10/rules_ts-v3.8.10.tar.gz",
)

# NOTE: aspect_rules_webpack exists for webpack, but it's incompatible with webpack v4.
http_archive(
    name = "aspect_rules_jest",
    sha256 = "6c7c9d043aae96c315bf2d77cd3bf4dc385bd080e3086edd17cada49747e81fa",
    strip_prefix = "rules_jest-0.25.2",
    url = "https://github.com/aspect-build/rules_jest/releases/download/v0.25.2/rules_jest-v0.25.2.tar.gz",
)

# Load gazelle. This lets us auto-generate BUILD.bazel files throughout the
# repo.
http_archive(
    name = "bazel_gazelle",
    sha256 = "92329a7dbb26d0beacc43da669211546ea6627582793f4dd5f28837fde3a5c08",
    urls = [
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.50.0/bazel-gazelle-v0.50.0.tar.gz",
    ],
)

# Go module dependencies are declared in MODULE.bazel via gazelle's bzlmod
# go_deps extension (sourced from //:go.mod). DEPS.bzl is retained only as an
# auxiliary record gazelle's update-repos writes to; it is not loaded here.

####### THIRD-PARTY DEPENDENCIES #######
# Below we need to call into various helper macros to pull dependencies for
# helper libraries like rules_go, rules_js, and rules_foreign_cc. However,
# calling into those helper macros can cause the build to pull from sources not
# under CRDB's control. To avoid this, we pre-emptively declare each repository
# as an http_archive/go_repository *before* calling into the macro where it
# would otherwise be defined. In doing so we "override" the URL the macro will
# point to.
#
# When upgrading any of these helper libraries, you will have to manually
# inspect the definition of the macro to see what's changed. If the helper
# library has defined any new dependencies, check whether we've already defined
# that repository somewhere (either in this file or in `DEPS.bzl`). If it is
# already defined somewhere, then add a note like "$REPO handled in DEPS.bzl"
# for future maintainers. Otherwise, mirror the .tar.gz and add an http_archive
# pointing to the mirror. For dependencies that were updated, check whether we
# need to pull a new version of that dependency and mirror it and update the URL
# accordingly.

###############################
# begin rules_go dependencies #
###############################

# For those rules_go dependencies that are NOT handled in DEPS.bzl, we point to
# CRDB mirrors.

# Ref: https://github.com/bazelbuild/rules_go/blob/master/go/private/repositories.bzl

# platforms handled in MODULE.bazel.

http_archive(
    name = "bazel_skylib",
    sha256 = "4ede85dfaa97c5662c3fb2042a7ac322d5f029fdc7a6b9daa9423b746e8e8fc0",
    strip_prefix = "bazelbuild-bazel-skylib-6a17363",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/bazelbuild-bazel-skylib-1.3.0-0-g6a17363.tar.gz",
    ],
)

# org_golang_x_sys handled in DEPS.bzl.
# org_golang_x_tools handled in DEPS.bzl.
# org_golang_x_xerrors handled in DEPS.bzl.

# rules_cc and its internal @cc_compatibility_proxy are set up via MODULE.bazel.

# org_golang_google_protobuf handled in DEPS.bzl.
# com_github_golang_protobuf handled in DEPS.bzl.
# com_github_mwitkow_go_proto_validators handled in DEPS.bzl.
# com_github_gogo_protobuf handled in DEPS.bzl.
# org_golang_google_genproto handled in DEPS.bzl.

# go_googleapis pinned to the same snapshot CRDB has always used, but without
# the upstream rules_go patches (removed in rules_go 0.60) or the CRDB-local
# BUILD.bazel patch (which no longer applies cleanly).
http_archive(
    name = "go_googleapis",
    sha256 = "ba694861340e792fd31cb77274eacaf6e4ca8bda97707898f41d8bebfd8a4984",
    strip_prefix = "googleapis-83c3605afb5a39952bf0a0809875d41cf2a558ca",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/googleapis-83c3605afb5a39952bf0a0809875d41cf2a558ca.zip",
    ],
)

# Required by go_googleapis BUILD files — creates @com_google_googleapis_imports
# with the language-specific rule aliases.
load("@go_googleapis//:repository_rules.bzl", "switched_rules_by_language")

switched_rules_by_language(
    name = "com_google_googleapis_imports",
    go = True,
    grpc = True,
)

# com_github_golang_mock handled in DEPS.bzl.

# Load the go dependencies and invoke them.
load(
    "@io_bazel_rules_go//go:deps.bzl",
    "go_download_sdk",
    "go_host_sdk",
    "go_local_sdk",
    "go_register_toolchains",
    "go_rules_dependencies",
)

# To point to a mirrored artifact, use:
#
go_download_sdk(
    name = "go_sdk",
    sdks = {
        "darwin_amd64": ("go1.26.2.darwin-amd64.tar.gz", "bc3f1500d9968c36d705442d90ba91addf9271665033748b82532682e90a7966"),
        "darwin_arm64": ("go1.26.2.darwin-arm64.tar.gz", "32af1522bf3e3ff3975864780a429cc0b41d190ec7bf90faa661d6d64566e7af"),
        "freebsd_amd64": ("go1.26.2.freebsd-amd64.tar.gz", "f271fd829a2a6b36fa1c72cdaafb18410a106da982c93a626d4e8b0fa0f0fa21"),
        "linux_amd64": ("go1.26.2.linux-amd64.tar.gz", "990e6b4bbba816dc3ee129eaeaf4b42f17c2800b88a2166c265ac1a200262282"),
        "linux_arm64": ("go1.26.2.linux-arm64.tar.gz", "c958a1fe1b361391db163a485e21f5f228142d6f8b584f6bef89b26f66dc5b23"),
        "windows_amd64": ("go1.26.2.windows-amd64.zip", "98eb3570bade15cb826b0909338df6cc6d2cf590bc39c471142002db3832b708"),
    },
    urls = ["https://go.dev/dl/{}"],
    version = "1.26.2",
)

# To point to a local SDK path, use the following instead. We'll call the
# directory into which you cloned the Go repository $GODIR[1]. You'll have to
# first run ./make.bash from $GODIR/src to pick up any custom changes.
#
# [1]: https://go.dev/doc/contribute#testing
#
#   go_local_sdk(
#       name = "go_sdk",
#       path = "<path to $GODIR>",
#   )

# To use your whatever your local SDK is, use the following instead:
#
#   go_host_sdk(name = "go_sdk")

go_rules_dependencies()

go_register_toolchains(nogo = "@com_github_cockroachdb_cockroach//:crdb_nogo")

###############################
# end rules_go dependencies #
###############################

###################################
# begin rules_js dependencies #
###################################

# Install rules_js dependencies

# bazel_skylib handled above.

# The rules_nodejs "core" module.
http_archive(
    name = "rules_nodejs",
    sha256 = "32894b914e4aed4245afdcece6fad94413f7f86eaefb863ff71e8ba6e5992b6b",
    strip_prefix = "rules_nodejs-6.7.3",
    url = "https://github.com/bazel-contrib/rules_nodejs/releases/download/v6.7.3/rules_nodejs-v6.7.3.tar.gz",
)

http_archive(
    name = "bazel_features",
    sha256 = "966c211ec42c4deb2af4c6dd6948408100b752f61753c97055bdac9bfb5cc0c7",
    strip_prefix = "bazel_features-1.41.0",
    url = "https://github.com/bazel-contrib/bazel_features/releases/download/v1.41.0/bazel_features-v1.41.0.tar.gz",
)

load("@bazel_features//:deps.bzl", "bazel_features_deps")

bazel_features_deps()

http_archive(
    name = "tar.bzl",
    sha256 = "fea07e17117bf264f78bd4fc522c6b07a843cdcd82c6dfbeafa5bdaff70ae198",
    strip_prefix = "tar.bzl-0.10.4",
    url = "https://github.com/bazel-contrib/tar.bzl/releases/download/v0.10.4/tar.bzl-v0.10.4.tar.gz",
)

http_archive(
    name = "yq.bzl",
    sha256 = "975dd1db2663d1809ae8f07ed00ef9b0e5396a58fb73777a84bac379667f6ea2",
    strip_prefix = "yq.bzl-0.3.4",
    url = "https://github.com/bazel-contrib/yq.bzl/releases/download/v0.3.4/yq.bzl-v0.3.4.tar.gz",
)

http_archive(
    name = "jq.bzl",
    sha256 = "21617eb71fb775a748ef5639131ab943ef39946bd2a4ce96ea60b03f985db0c5",
    strip_prefix = "jq.bzl-0.4.0",
    url = "https://github.com/bazel-contrib/jq.bzl/releases/download/v0.4.0/jq.bzl-v0.4.0.tar.gz",
)

http_archive(
    name = "aspect_tools_telemetry",
    sha256 = "3d6372792dd0654b2e77806bf9b358e06706234f179132575a178d0ce7312790",
    strip_prefix = "tools_telemetry-0.3.3",
    url = "https://github.com/aspect-build/tools_telemetry/releases/download/v0.3.3/tools_telemetry-v0.3.3.tar.gz",
)

http_archive(
    name = "aspect_tools_telemetry_report",
    sha256 = "3d6372792dd0654b2e77806bf9b358e06706234f179132575a178d0ce7312790",
    strip_prefix = "tools_telemetry-0.3.3",
    url = "https://github.com/aspect-build/tools_telemetry/releases/download/v0.3.3/tools_telemetry-v0.3.3.tar.gz",
)

http_archive(
    name = "protobuf",
    repo_mapping = {
        "@proto_bazel_features": "@bazel_features",
    },
    sha256 = "687e98a471973b5c5fd711750c40b8b82c0ade33f649db65e00b290f29345a2b",
    strip_prefix = "protobuf-33.4",
    url = "https://github.com/protocolbuffers/protobuf/releases/download/v33.4/protobuf-33.4.bazel.tar.gz",
)

http_archive(
    name = "bazel_lib",
    sha256 = "3d62bf30b95b71a566d9ebca50ee78d370b12522d244235b41166f65b142705d",
    strip_prefix = "bazel-lib-3.2.2",
    url = "https://github.com/bazel-contrib/bazel-lib/releases/download/v3.2.2/bazel-lib-v3.2.2.tar.gz",
)

# Load custom toolchains.
load("//build/toolchains:REPOSITORIES.bzl", "toolchain_dependencies")

toolchain_dependencies()

# Configure nodeJS.
load("//build:nodejs.bzl", "declare_nodejs_repos")

declare_nodejs_repos()

# NOTE: The version is expected to match up to what version of typescript we
# use for all packages in pkg/ui.
# TODO(ricky): We should add a lint check to ensure it does match.
load("@aspect_rules_ts//ts/private:npm_repositories.bzl", ts_http_archive = "http_archive_version")

ts_http_archive(
    name = "npm_typescript",
    urls = ["https://storage.googleapis.com/cockroach-npm-deps/typescript/-/typescript-{}.tgz"],
    version = "4.2.4",
)

# NOTE: The version is expected to match up to what version we use in db-console.
# TODO(ricky): We should add a lint check to ensure it does match.
load("@aspect_rules_js_npm_translate//npm:repositories.bzl", "npm_import", "npm_translate_lock")

npm_import(
    name = "pnpm",
    # Declare an @pnpm//:pnpm rule that can be called externally.
    # Copied from https://github.com/aspect-build/rules_js/blob/14724d9b27b2c45f088aa003c091cbe628108170/npm/private/pnpm_repository.bzl#L27-L30
    extra_build_content = "\n".join([
        """load("@aspect_rules_js//js:defs.bzl", "js_binary")""",
        """js_binary(name = "pnpm", entry_point = "package/dist/pnpm.cjs", visibility = ["//visibility:public"])""",
    ]),
    integrity = "sha512-W6elL7Nww0a/MCICkzpkbxW6f99TQuX4DuJoDjWp39X08PKDkEpg4cgj3d6EtgYADcdQWl/eM8NdlLJVE3RgpA==",
    package = "pnpm",
    url = "https://storage.googleapis.com/cockroach-npm-deps/pnpm/-/pnpm-8.5.1.tgz",
    version = "8.5.1",
)

load("@aspect_rules_js_npm_translate//js:repositories.bzl", "rules_js_dependencies")

rules_js_dependencies()

npm_translate_lock(
    name = "npm",
    data = [
        "//pkg/ui:package.json",
        "//pkg/ui:pnpm-workspace.yaml",
        "//pkg/ui/patches:topojson@3.0.2.patch",
        "//pkg/ui/workspaces/cluster-ui:package.json",
        "//pkg/ui/workspaces/db-console:package.json",
        "//pkg/ui/workspaces/db-console/src/js:package.json",
        "//pkg/ui/workspaces/e2e-tests:package.json",
        "//pkg/ui/workspaces/eslint-plugin-crdb:package.json",
    ],
    npmrc = "//pkg/ui:.npmrc.bazel",
    patch_args = {
        "*": ["-p1"],
    },
    pnpm_lock = "//pkg/ui:pnpm-lock.yaml",
    verify_node_modules_ignored = "//:.bazelignore",
)

npm_import(
    name = "npm_playwright_core",
    integrity = "sha512-hutraynyn31F+Bifme+Ps9Vq59hKuUCz7H1kDOcBs+2oGguKkWTU50bBWrtz34OUWmIwpBTWDxaRPXrIXkgvmQ==",
    package = "playwright-core",
    url = "https://registry.npmjs.org/playwright-core/-/playwright-core-1.56.1.tgz",
    version = "1.56.1",
)

npm_import(
    name = "npm_playwright",
    integrity = "sha512-aFi5B0WovBHTEvpM3DzXTUaeN6eN0qWnTkKx4NQaH4Wvcmc153PdaY2UBdSYKaGYw+UyWXSVyxDUg5DoPEttjw==",
    package = "playwright",
    url = "https://registry.npmjs.org/playwright/-/playwright-1.56.1.tgz",
    version = "1.56.1",
)

load("@npm//:repositories.bzl", "npm_repositories")

npm_repositories()

#################################
# end rules_js dependencies #
#################################

##############################
# begin gazelle dependencies #
##############################

# Load gazelle dependencies.
load(
    "@bazel_gazelle//:deps.bzl",
    "gazelle_dependencies",
    "go_repository",
)

# Ref: https://github.com/bazelbuild/bazel-gazelle/blob/master/deps.bzl

# bazel_skylib handled above.
# co_honnef_go_tools handled in DEPS.bzl.

# keep
http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "f3b800e9f6ca60bdef3709440f393348f7c18a29f30814288a7326285c80aab9",
    strip_prefix = "buildtools-8.5.1",
    urls = [
        "https://github.com/bazelbuild/buildtools/archive/refs/tags/v8.5.1.tar.gz",
    ],
)

# com_github_bazelbuild_rules_go handled in DEPS.bzl.

# keep
go_repository(
    name = "com_github_bmatcuk_doublestar_v4",
    importpath = "github.com/bmatcuk/doublestar/v4",
    sha256 = "5d178e61fe67b3ae3ea46f023b2fbfaf0400e6ee74fe5cef1074690305a3f4f6",
    strip_prefix = "doublestar-4.10.0",
    urls = [
        "https://github.com/bmatcuk/doublestar/archive/refs/tags/v4.10.0.tar.gz",
    ],
)

# com_github_burntsushi_toml handled in DEPS.bzl.
# com_github_census_instrumentation_opencensus_proto handled in DEPS.bzl.
# com_github_chzyer_logex handled in DEPS.bzl.
# com_github_chzyer_readline handled in DEPS.bzl.
# com_github_chzyer_test handled in DEPS.bzl.
# com_github_client9_misspell handled in DEPS.bzl.
# com_github_davecgh_go_spew handled in DEPS.bzl.
# com_github_envoyproxy_go_control_plane handled in DEPS.bzl.
# com_github_envoyproxy_protoc_gen_validate handled in DEPS.bzl.
# com_github_fsnotify_fsnotify handled in DEPS.bzl.
# com_github_golang_glog handled in DEPS.bzl.
# com_github_golang_mock handled in DEPS.bzl.
# com_github_golang_protobuf handled in DEPS.bzl.
# com_github_google_go_cmp handled in DEPS.bzl.
# com_github_pelletier_go_toml handled in DEPS.bzl.
# com_github_pmezard_go_difflib handled in DEPS.bzl.
# com_github_prometheus_client_model handled in DEPS.bzl.
# com_github_yuin_goldmark handled in DEPS.bzl.
# com_google_cloud_go handled in DEPS.bzl.
# in_gopkg_check_v1 handled in DEPS.bzl.
# in_gopkg_yaml_v2 handled in DEPS.bzl.

# keep
go_repository(
    name = "net_starlark_go",
    importpath = "go.starlark.net",
    sha256 = "a35c6468e0e0921833a63290161ff903295eaaf5915200bbce272cbc8dfd1c1c",
    strip_prefix = "google-starlark-go-e043a3d",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/google-starlark-go-e043a3d.tar.gz",
    ],
)

# org_golang_google_genproto handled in DEPS.bzl.
# org_golang_google_grpc handled in DEPS.bzl.
# org_golang_google_protobuf handled in DEPS.bzl.
# org_golang_x_crypto handled in DEPS.bzl.
# org_golang_x_exp handled in DEPS.bzl.
# org_golang_x_lint handled in DEPS.bzl.
# org_golang_x_mod handled in DEPS.bzl.
# org_golang_x_net handled in DEPS.bzl.
# org_golang_x_oauth2 handled in DEPS.bzl.
# org_golang_x_sync handled in DEPS.bzl.
# org_golang_x_sys handled in DEPS.bzl.
# org_golang_x_text handled in DEPS.bzl.
# org_golang_x_tools handled in DEPS.bzl.
# org_golang_x_xerrors handled in DEPS.bzl.

gazelle_dependencies()

############################
# end gazelle dependencies #
############################

###############################
# begin protobuf dependencies #
###############################

# Load the protobuf dependency.
#
# Ref: https://github.com/bazelbuild/rules_go/blob/0.19.0/go/workspace.rst#proto-dependencies
#      https://github.com/bazelbuild/bazel-gazelle/issues/591
#      https://github.com/protocolbuffers/protobuf/blob/main/protobuf_deps.bzl
http_archive(
    name = "com_google_protobuf",
    sha256 = "79cc6d09d02706c5a73e900ea842b5b3dae160f371b6654774947fe781851423",
    strip_prefix = "protobuf-27.5",
    urls = [
        "https://github.com/protocolbuffers/protobuf/releases/download/v27.5/protobuf-27.5.tar.gz",
    ],
)

http_archive(
    name = "zlib",
    build_file = "@com_google_protobuf//:third_party/zlib.BUILD",
    sha256 = "c3e5e9fdd5004dcb542feda5ee4f0ff0744628baf8ed2dd5d66f8ca1197cb1a1",
    strip_prefix = "zlib-1.2.11",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/zlib/zlib-1.2.11.tar.gz",
    ],
)

# NB: we don't use six for anything. We're just including it here so we don't
# incidentally pull it from pypi.
http_archive(
    name = "six",
    build_file = "@com_google_protobuf//:six.BUILD",
    sha256 = "105f8d68616f8248e24bf0e9372ef04d3cc10104f1980f54d57b2ce73a5ad56a",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/python/six-1.10.0.tar.gz",
    ],
)

# rules_cc handled above.

# NB: we don't use rules_java for anything. We're just including it here so we
# don't incidentally pull it from github.
# bazel_features and rules_java handled in MODULE.bazel.

http_archive(
    name = "rules_proto",
    sha256 = "88b0a90433866b44bb4450d4c30bc5738b8c4f9c9ba14e9661deb123f56a833d",
    strip_prefix = "rules_proto-b0cc14be5da05168b01db282fe93bdf17aa2b9f4",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/rules_proto-b0cc14be5da05168b01db282fe93bdf17aa2b9f4.tar.gz",
    ],
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

# Go dependencies are handled entirely by the bzlmod `go_deps` extension in
# MODULE.bazel (sourced from //:go.mod).  The legacy DEPS.bzl is no longer
# loaded here to avoid dual-declaration conflicts at link time.

protobuf_deps()

#############################
# end protobuf dependencies #
#############################

# Loading c-deps third party dependencies.
load("//c-deps:REPOSITORIES.bzl", "c_deps")

c_deps()

#######################################
# begin rules_foreign_cc dependencies #
#######################################

# Load the bazel utility that lets us build C/C++ projects using
# cmake/make/etc. We point to our fork which adds BSD support
# (https://github.com/bazelbuild/rules_foreign_cc/pull/387) and sysroot
# support (https://github.com/bazelbuild/rules_foreign_cc/pull/532).
#
# TODO(irfansharif): Point to an upstream SHA once maintainers pick up the
# aforementioned PRs.
#
# Ref: https://github.com/bazelbuild/rules_foreign_cc/blob/main/foreign_cc/repositories.bzl
http_archive(
    name = "rules_foreign_cc",
    sha256 = "272ac2cde4efd316c8d7c0140dee411c89da104466701ac179286ef5a89c7b58",
    strip_prefix = "cockroachdb-rules_foreign_cc-6f7f1b1",
    urls = [
        # As of commit 6f7f1b1c6f911db5706c2fcbb3d5669d95974a34 (release 0.7.0 plus a couple patches)
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/cockroachdb-rules_foreign_cc-6f7f1b1.tar.gz",
    ],
)

load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")

# bazel_skylib is handled above.

rules_foreign_cc_dependencies(
    register_built_tools = False,
    register_default_tools = False,
    register_preinstalled_tools = True,
)

#####################################
# end rules_foreign_cc dependencies #
#####################################

################################
# begin rules_pkg dependencies #
################################

http_archive(
    name = "rules_pkg",
    sha256 = "8a298e832762eda1830597d64fe7db58178aa84cd5926d76d5b744d6558941c2",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/rules_pkg-0.7.0.tar.gz",
    ],
)
# Ref: https://github.com/bazelbuild/rules_pkg/blob/main/pkg/deps.bzl

# bazel_skylib handled above.
http_archive(
    name = "rules_python",
    sha256 = "b6d46438523a3ec0f3cead544190ee13223a52f6a6765a29eae7b7cc24cc83a0",
    urls = ["https://storage.googleapis.com/public-bazel-artifacts/bazel/rules_python-0.1.0.tar.gz"],
)

http_archive(
    name = "rules_license",
    sha256 = "4865059254da674e3d18ab242e21c17f7e3e8c6b1f1421fffa4c5070f82e98b5",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/rules_license-0.0.1.tar.gz",
    ],
)

load("@rules_pkg//pkg:deps.bzl", "rules_pkg_dependencies")

rules_pkg_dependencies()

##############################
# end rules_pkg dependencies #
##############################

register_toolchains(
    "//build/toolchains:cross_x86_64_linux_toolchain",
    "//build/toolchains:cross_x86_64_linux_arm_toolchain",
    "//build/toolchains:cross_x86_64_s390x_toolchain",
    "//build/toolchains:cross_x86_64_macos_toolchain",
    "//build/toolchains:cross_x86_64_macos_arm_toolchain",
    "//build/toolchains:cross_x86_64_windows_toolchain",
    "//build/toolchains:cross_arm64_linux_toolchain",
    "//build/toolchains:cross_arm64_linux_arm_toolchain",
    "//build/toolchains:cross_arm64_s390x_toolchain",
    "//build/toolchains:cross_arm64_windows_toolchain",
    "//build/toolchains:cross_arm64_macos_toolchain",
    "//build/toolchains:cross_arm64_macos_arm_toolchain",
    "//build/toolchains:node_freebsd_toolchain",
    "@copy_directory_toolchains//:darwin_amd64_toolchain",
    "@copy_directory_toolchains//:darwin_arm64_toolchain",
    "@copy_directory_toolchains//:freebsd_amd64_toolchain",
    "@copy_directory_toolchains//:linux_amd64_toolchain",
    "@copy_directory_toolchains//:linux_arm64_toolchain",
    "@copy_directory_toolchains//:windows_amd64_toolchain",
    "@copy_directory_toolchains_legacy//:darwin_amd64_toolchain",
    "@copy_directory_toolchains_legacy//:darwin_arm64_toolchain",
    "@copy_directory_toolchains_legacy//:freebsd_amd64_toolchain",
    "@copy_directory_toolchains_legacy//:linux_amd64_toolchain",
    "@copy_directory_toolchains_legacy//:linux_arm64_toolchain",
    "@copy_directory_toolchains_legacy//:windows_amd64_toolchain",
    "@copy_to_directory_toolchains//:darwin_amd64_toolchain",
    "@copy_to_directory_toolchains//:darwin_arm64_toolchain",
    "@copy_to_directory_toolchains//:freebsd_amd64_toolchain",
    "@copy_to_directory_toolchains//:linux_amd64_toolchain",
    "@copy_to_directory_toolchains//:linux_arm64_toolchain",
    "@copy_to_directory_toolchains//:windows_amd64_toolchain",
    "@copy_to_directory_toolchains_legacy//:darwin_amd64_toolchain",
    "@copy_to_directory_toolchains_legacy//:darwin_arm64_toolchain",
    "@copy_to_directory_toolchains_legacy//:freebsd_amd64_toolchain",
    "@copy_to_directory_toolchains_legacy//:linux_amd64_toolchain",
    "@copy_to_directory_toolchains_legacy//:linux_arm64_toolchain",
    "@copy_to_directory_toolchains_legacy//:windows_amd64_toolchain",
    "@nodejs_toolchains//:darwin_amd64_toolchain",
    "@nodejs_toolchains//:darwin_arm64_toolchain",
    # NB: The freebsd node toolchain is above as //build/toolchains:node_freebsd_toolchain
    "@nodejs_toolchains//:linux_amd64_toolchain",
    "@nodejs_toolchains//:linux_arm64_toolchain",
    "@nodejs_toolchains//:windows_amd64_toolchain",
)

http_archive(
    name = "bazel_gomock",
    sha256 = "692421b0c5e04ae4bc0bfff42fb1ce8671fe68daee2b8d8ea94657bb1fcddc0a",
    strip_prefix = "bazel_gomock-fde78c91cf1783cc1e33ba278922ba67a6ee2a84",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/bazel_gomock-fde78c91cf1783cc1e33ba278922ba67a6ee2a84.tar.gz",
    ],
)

http_archive(
    name = "com_github_cockroachdb_sqllogictest",
    build_file_content = """
filegroup(
    name = "testfiles",
    srcs = glob(["test/**/*.test"]),
    visibility = ["//visibility:public"],
)""",
    sha256 = "f7e0d659fbefb65f32d4c5d146cba4c73c43e0e96f9b217a756c82be17451f97",
    strip_prefix = "sqllogictest-96138842571462ed9a697bff590828d8f6356a2f",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/bazel/sqllogictest-96138842571462ed9a697bff590828d8f6356a2f.tar.gz",
    ],
)

http_archive(
    name = "railroadjar",
    build_file_content = """exports_files(["rr.war"])""",
    sha256 = "d2791cd7a44ea5be862f33f5a9b3d40aaad9858455828ebade7007ad7113fb41",
    urls = [
        "https://storage.googleapis.com/public-bazel-artifacts/java/railroad/rr-1.63-java8.zip",
    ],
)

load("//build/bazelutil:repositories.bzl", "distdir_repositories")

distdir_repositories()
