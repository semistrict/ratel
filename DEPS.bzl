load("@bazel_gazelle//:deps.bzl", "go_repository")

# PRO-TIP: You can inject temorary changes to any of these dependencies by
# by pointing to an alternate remote to clone from. Delete the `sha256`,
# `strip_prefix`, and `urls` parameters, and add `vcs = "git"` as well as a
# custom `remote` and `commit`. For example:
#     go_repository(
#        name = "com_github_cockroachdb_sentry_go",
#        build_file_proto_mode = "disable_global",
#        importpath = "github.com/cockroachdb/sentry-go",
#        vcs = "git",
#        remote = "https://github.com/rickystewart/sentry-go",  # Custom fork.
#        commit = "6c8e10aca9672de108063d4953399bd331b54037",  # Custom commit.
#    )
# The `remote` can be EITHER a URL, or an absolute local path to a clone, such
# as `/Users/ricky/go/src/github.com/cockroachdb/sentry-go`. Bazel will clone
# from the remote and check out the commit you specify.

def go_deps():
    # NOTE: We ensure that we pin to these specific dependencies by calling
    # this function FIRST, before calls to pull in dependencies for
    # third-party libraries (e.g. rules_go, gazelle, etc.)
    go_repository(
        name = "co_honnef_go_tools",
        build_file_proto_mode = "disable_global",
        importpath = "honnef.co/go/tools",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:co_honnef_go_tools.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:o/n5/K5gXqk8Gozvs2cnL0F2S1/g1vcGCAx2vETjITw=",
        version = "v0.4.3",
    )
    go_repository(
        name = "com_github_abbot_go_http_auth",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/abbot/go-http-auth",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9ZqcMQ0fB+ywKACVjGfZM4C7Uq9D5rq0iSmwIjX187k=",
        version = "v0.4.1-0.20181019201920-860ed7f246ff",
    )
    go_repository(
        name = "com_github_abourget_teamcity",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/abourget/teamcity",
        replace = "github.com/cockroachdb/teamcity",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UAqRo5xPCyTtZznAJ9iPVpDUFxGI0a6QWtQ8E+zwJRg=",
        version = "v0.0.0-20180905144921-8ca25c33eb11",
    )
    go_repository(
        name = "com_github_aclements_go_gg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aclements/go-gg",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:E5Dzlk3akC+T2Zj1LBHgfPK1y8YWgLDnNDRmG+tpSKw=",
        version = "v0.0.0-20170118225347-6dbb4e4fefb0",
    )
    go_repository(
        name = "com_github_aclements_go_moremath",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aclements/go-moremath",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xlwdaKcTNVW4PtpQb8aKA4Pjy0CdJHEqvFbAnvR5m2g=",
        version = "v0.0.0-20210112150236-f10218a38794",
    )
    go_repository(
        name = "com_github_afex_hystrix_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/afex/hystrix-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rFw4nCn9iMW+Vajsk51NtYIcwSTkXr+JGrMd36kTDJw=",
        version = "v0.0.0-20180502004556-fa1af6a1f4f5",
    )
    go_repository(
        name = "com_github_agnivade_levenshtein",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/agnivade/levenshtein",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3oJU7J3FGFmyhn8KHjmVaZCN5hxTr7GxgRue+sxIXdQ=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_ajg_form",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ajg/form",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:t9c7v8JUKu/XxOGBU0yjNpaMloxGEJhUkqFRq0ibGeU=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_ajstarks_svgo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ajstarks/svgo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:slYM766cy2nI3BwyRiyQj/Ud48djTMtMebDqepE95rw=",
        version = "v0.0.0-20211024235047-1546f124cd8b",
    )
    go_repository(
        name = "com_github_akavel_rsrc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/akavel/rsrc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zjWn7ukO9Kc5Q62DOJCcxGpXC18RawVtYAGdz2aLlfw=",
        version = "v0.8.0",
    )
    go_repository(
        name = "com_github_alecthomas_kingpin_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/kingpin/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ANLJcKmQm4nIaog7xdr/id6FM6zm5hHnfZrvtKPxqGg=",
        version = "v2.3.1",
    )
    go_repository(
        name = "com_github_alecthomas_template",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/template",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JYp7IbQjafoB+tBA3gMyHYHrpOtNuDiK/uB5uXxq5wM=",
        version = "v0.0.0-20190718012654-fb15b899a751",
    )
    go_repository(
        name = "com_github_alecthomas_units",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alecthomas/units",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:s6gZFSlWYmbqAuRjVTiNNhvNRfY2Wxp9nhfyel4rklc=",
        version = "v0.0.0-20211218093645-b94a6e3cc137",
    )
    go_repository(
        name = "com_github_alessio_shellescape",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alessio/shellescape",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:V7yhSDDn8LP4lc4jS8pFkt0zCnzVJlG5JXy9BVKJUX0=",
        version = "v1.4.1",
    )
    go_repository(
        name = "com_github_alexbrainman_sspi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alexbrainman/sspi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Kk6a4nehpJ3UuJRqlA3JxYxBZEqCeOmATOvrbT4p9RA=",
        version = "v0.0.0-20210105120005-909beea2cc74",
    )
    go_repository(
        name = "com_github_alexflint_go_filemutex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/alexflint/go-filemutex",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AMzIhMUqU3jMrZiTuW0zkYeKlKDAFD+DG20IoO421/Y=",
        version = "v0.0.0-20171022225611-72bdc8eae2ae",
    )
    go_repository(
        name = "com_github_andreasbriese_bbloom",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/AndreasBriese/bbloom",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HD8gA2tkByhMAwYaFAX9w2l7vxvBQ5NMoxDrkhqhtn4=",
        version = "v0.0.0-20190306092124-e2d15f34fcf9",
    )
    go_repository(
        name = "com_github_andreyvit_diff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andreyvit/diff",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bvNMNQO63//z+xNgfBlViaCIJKLlCJ6/fmUseuG0wVQ=",
        version = "v0.0.0-20170406064948-c7f18ee00883",
    )
    go_repository(
        name = "com_github_andy_kimball_arenaskl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andy-kimball/arenaskl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vCvyXiLsgAs7qgclk56iBTJQ+gdfiVuzfe5T6sVBL+w=",
        version = "v0.0.0-20200617143215-f701008588b9",
    )
    go_repository(
        name = "com_github_andybalholm_brotli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andybalholm/brotli",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8uQZIdzKmjc/iuPu7O2ioW48L81FgatrcpfFmiq/cCs=",
        version = "v1.0.5",
    )
    go_repository(
        name = "com_github_andybalholm_cascadia",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andybalholm/cascadia",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vuRCkM5Ozh/BfmsaTm26kbjm0mIOM3yS5Ek/F5h18aE=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_andygrunwald_go_jira",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/andygrunwald/go-jira",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7GT/3qhar2dGJ0kq8w0d63liNyHOnxZsUZ9Pe4+AKBI=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_github_antihax_optional",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/antihax/optional",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xK2lYat7ZLaVVcIuj82J8kIro4V6kDe0AUDFboUCwcg=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_aokoli_goutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aokoli/goutils",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7fpzNGoJ3VA8qcrm++XEE1QUe0mIwNeLa02Nwq7RDkg=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_apache_arrow_go_arrow",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/apache/arrow/go/arrow",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FSqtT0UCktIlSU19mxj0YE5HK3HOO4IFMU9BpOif/7A=",
        version = "v0.0.0-20200923215132-ac86123a3f01",
    )
    go_repository(
        name = "com_github_apache_thrift",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/apache/thrift",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qEy6UW60iVOlUy+b9ZR0d5WzUWYGOo4HfopoyBaNmoY=",
        version = "v0.16.0",
    )
    go_repository(
        name = "com_github_araddon_dateparse",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/araddon/dateparse",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FxWPpzIjnTlhPwqqXc4/vE0f7GvRjuAsbW+HOIe8KnA=",
        version = "v0.0.0-20210429162001-6b43995a97de",
    )
    go_repository(
        name = "com_github_armon_circbuf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/circbuf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QEF07wC0T1rKkctt1RINW/+RMTVmiwxETico2l3gxJA=",
        version = "v0.0.0-20150827004946-bbbad097214e",
    )
    go_repository(
        name = "com_github_armon_consul_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/consul-api",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:G1bPvciwNyF7IUmKXNt9Ak3m6u9DE1rF+RmtIkBpVdA=",
        version = "v0.0.0-20180202201655-eb2c6b5be1b6",
    )
    go_repository(
        name = "com_github_armon_go_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/go-metrics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:a9F4rlj7EWWrbj7BYw8J8+x+ZZkJeqzNyRk8hdPF+ro=",
        version = "v0.3.3",
    )
    go_repository(
        name = "com_github_armon_go_radix",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/go-radix",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:F4z6KzEeeQIMeLFa97iZU6vupzoecKdU5TX24SNppXI=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_armon_go_socks5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/armon/go-socks5",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0CwZNZbxp69SHPdPJAN/hZIm0C4OItdklCFmMRWYpio=",
        version = "v0.0.0-20160902184237-e75332964ef5",
    )
    go_repository(
        name = "com_github_aryann_difflib",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aryann/difflib",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pv34s756C4pEXnjgPfGYgdhg/ZdajGhyOvzx8k+23nw=",
        version = "v0.0.0-20170710044230-e206f873d14a",
    )
    go_repository(
        name = "com_github_asaskevich_govalidator",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/asaskevich/govalidator",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:46PFijGLmAjMPwCCCo7Jf0W6f9slllCkkv7vyc1yOSg=",
        version = "v0.0.0-20200907205600-7a23bdc65eef",
    )
    go_repository(
        name = "com_github_atotto_clipboard",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/atotto/clipboard",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EH0zSVneZPSuFR11BlR9YppQTVDbh5+16AmcJi4g1z4=",
        version = "v0.1.4",
    )
    go_repository(
        name = "com_github_aws_aws_lambda_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-lambda-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:SuCy7H3NLyp+1Mrfp+m80jcbi9KYWAs9/BXwppwRDzY=",
        version = "v1.13.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:I+Q6cLctkFyMMrKukcDnj+i2kjrQ37LGiOM6xmsxC48=",
        version = "v1.40.37",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:shN7NlnVzvDUgPQ+1rLMSxY8OWRNDRYtiqe0p/PgrhY=",
        version = "v1.17.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_config",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/config",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5AlQD0jhVXlGzwo+VORKiUuogkG7pQcLJNzIzK7eodw=",
        version = "v1.15.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_credentials",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/credentials",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RQQ5fzclAKJyY5TvF+fkjJEwzK4hnxQCLOu5JXzDmQo=",
        version = "v1.11.2",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_feature_ec2_imds",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/feature/ec2/imds",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LWPg5zjHV9oz/myQr4wMs0gi4CjnDN/ILmyZUFYXZsU=",
        version = "v1.12.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_internal_configsources",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/internal/configsources",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:I3cakv2Uy1vNmmhRQmFptYDxOvBnwCdNwyw63N0RaRU=",
        version = "v1.1.27",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_internal_endpoints_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/internal/endpoints/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5NbbMrIzmUn/TXFqAle6mgrH5m9cOvMLRGL7pnG8tRE=",
        version = "v2.4.21",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_internal_ini",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/internal/ini",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:by9P+oy3P/CwggN4ClnW2D4oL91QV7pBzBICi1chZvQ=",
        version = "v1.3.10",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_databasemigrationservice",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/databasemigrationservice",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1OIa4zpY43n6Q5IX9b5mZWeq/KTV5243tXq5ABWlufI=",
        version = "v1.18.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_ec2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/ec2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dfWleW7/a3+TR6qJynYZsaovCEStQOep5x+BxkiBDhc=",
        version = "v1.34.0",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_iam",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/iam",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wllKL2fLtvfaNAVbXKMRmM/mD1oDNw0hXmDn8mE/6Us=",
        version = "v1.18.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_internal_presigned_url",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/internal/presigned-url",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Gh1Gpyh01Yvn7ilO/b/hr01WgNpaszfbKMUgqM186xQ=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_rds",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/rds",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:o5uszX76RaDFxNvf6b8qZ3GF55EdFoPH8GnrROjtL4k=",
        version = "v1.18.4",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_secretsmanager",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/secretsmanager",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QDVKb2VpuwzIslzshumxksayV5GkpqT+rkVvdPVrA9E=",
        version = "v1.18.2",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_sso",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/sso",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:frW4ikGcxfAEDfmQqWgMLp+F1n4nRo9sF39OcIb5BkQ=",
        version = "v1.11.3",
    )
    go_repository(
        name = "com_github_aws_aws_sdk_go_v2_service_sts",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/aws-sdk-go-v2/service/sts",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cJGRyzCSVwZC7zZZ1xbx9m32UnrKydRYhOvcD1NYP9Q=",
        version = "v1.16.3",
    )
    go_repository(
        name = "com_github_aws_smithy_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aws/smithy-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hgz0X/DX0dGqTYpGALqXJoRKRj5oQ7150i5FdTePzO8=",
        version = "v1.13.5",
    )
    go_repository(
        name = "com_github_axiomhq_hyperloglog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/axiomhq/hyperloglog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:190ugM9MsyFauTkR/UqcHG/mn5nmFe6SvHJqEHIrtrA=",
        version = "v0.0.0-20181223111420-4b99d0c2c99e",
    )
    go_repository(
        name = "com_github_aymanbagabas_go_osc52",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aymanbagabas/go-osc52",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DTwqENW7X9arYimJrPeGZcV0ln14sGMt3pHZspWD+Mg=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_aymerick_douceur",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aymerick/douceur",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Mv+mAeH1Q+n9Fr+oyamOlAkUNPWPlA8PPGR0QAaYuPk=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_aymerick_raymond",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/aymerick/raymond",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Ppm0npCCsmuR9oQaBtRuZcmILVE74aXE+AmrJj8L2ns=",
        version = "v2.0.3-0.20180322193309-b565731e1464+incompatible",
    )
    go_repository(
        name = "com_github_azure_azure_pipeline_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-pipeline-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OLBdZJ3yvOn2MezlWvbrBMTEUQC72zAftRZOMdj5HYo=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TKQ3ieyB0vVKkF6t9dsWbMjq56O1xU3eh3Ec09v6ajM=",
        version = "v57.1.0+incompatible",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_azcore",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/azcore",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VuHAcMq8pU1IWNT/m5yRaGqbK0BiQKHT8X4DTp9CHdI=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_azidentity",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/azidentity",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QkAcEIAKbNL4KoFr4SathZPhDhF4mVwpBMFlYjyAqy8=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_internal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/internal",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Oj853U9kG+RLTCQXpjvOnrv0WaZHxgmZz1TlLywgOPY=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_keyvault_azkeys",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/keyvault/azkeys",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TOFrNxfjslms5nLLIMjW7N0+zSALX4KiGsptmpb16AA=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_keyvault_internal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/keyvault/internal",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Lg6BW0VPmCwcMlvOviL3ruHFO+H9tZNqscK0AeuFjGM=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_azure_azure_sdk_for_go_sdk_storage_azblob",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YvQv9Mz6T8oR5ypQOL6erY0Z5t71ak1uHV4QFokCOZk=",
        version = "v0.6.1",
    )
    go_repository(
        name = "com_github_azure_azure_storage_blob_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/azure-storage-blob-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:53qhf0Oxa0nOjgbDeeYPUeyiNmafAFEY95rZLK0Tj6o=",
        version = "v0.8.0",
    )
    go_repository(
        name = "com_github_azure_go_ansiterm",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-ansiterm",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UQHMgLO+TxOElx5B5HZ4hJQsoJ/PvUvKRhJHDQXO8P8=",
        version = "v0.0.0-20210617225240-d185dfc1b5a1",
    )
    go_repository(
        name = "com_github_azure_go_autorest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:V5VMDjClD3GiElqLWO7mz2MxNAK/vTfRHdAubSIPRgs=",
        version = "v14.2.0+incompatible",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:s8H1PbCZSqg/DH7JMlOz6YMig6htWLNPsjDdlLqCx3M=",
        version = "v0.11.20",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_adal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/adal",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:X+p2GF0GWyOiSmqohIaEeuNFNDY4I4EOlVuUQvFdWMk=",
        version = "v0.9.15",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_azure_auth",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/azure/auth",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TzPg6B6fTZ0G1zBf3T54aI7p3cAT6u//TOXGPmFMOXg=",
        version = "v0.5.8",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_azure_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/azure/cli",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DOhB+nXkF7LN0JfBGB5YtCF6QLK8mLe4psaHF7ZQEKM=",
        version = "v0.4.3",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_date",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/date",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7gUk1U5M/CQbp9WoqinNzJar+8KY+LPI6wiWrP/myHw=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_mocks",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/mocks",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:K0laFcLE6VLTOwNgSxaGbUcLPuGXlNkbVvq4cW4nIHk=",
        version = "v0.4.1",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_to",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/to",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oXVqrxakqqV1UZdSazDOPOLvOIz+XA683u8EctwboHk=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_azure_go_autorest_autorest_validation",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/autorest/validation",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AgyqjAd94fwNAoTjl/WQXg4VvFeRFpO+UhNyRXqF1ac=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_azure_go_autorest_logger",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/logger",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IG7i4p/mDa2Ce4TRyAO8IHnVhAVF3RFU+ZtXWSmf4Tg=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_azure_go_autorest_tracing",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Azure/go-autorest/tracing",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TYi4+3m5t6K48TGI9AUdb+IzbnSxvnvUMfuitfgcfuo=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_azuread_microsoft_authentication_library_for_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/AzureAD/microsoft-authentication-library-for-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BWe8a+f/t+7KY7zH2mqygeUD0t8hNFXe08p1Pb3/jKE=",
        version = "v0.5.1",
    )
    go_repository(
        name = "com_github_bazelbuild_remote_apis",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bazelbuild/remote-apis",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cEFRynjrFOjUj9ZQj/ubiVbKPUcMG2kpMIbQkKGYlcI=",
        version = "v0.0.0-20200708200203-1252343900d9",
    )
    go_repository(
        name = "com_github_bazelbuild_rules_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bazelbuild/rules_go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2F449QezDZcVW6Jt+kSs8Htd/YI3EXMcvd0aNfVNCI4=",
        version = "v0.26.0",
    )
    go_repository(
        name = "com_github_benbjohnson_clock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/benbjohnson/clock",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Q92kusRqC1XV2MjkWETPvjJVqKetz1OzxZB7mHJLju8=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_benbjohnson_immutable",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/benbjohnson/immutable",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EVv7H1ju7cDg/a8HUF4hAH4DBrMJh6RWWFwq9JfoO9I=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_benbjohnson_tmpl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/benbjohnson/tmpl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:T5QPGJD0W6JJxyEEAlVnX3co/IkUrfHen1/42nlgAHo=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_beorn7_perks",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/beorn7/perks",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VlbKKnNfV8bJzeqoa4cOKqO6bYr3WgKZxO8Z16+hsOM=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_bgentry_go_netrc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bgentry/go-netrc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xDfNPAt8lFiC1UJrqV3uuy861HCTo708pDMbjHHdCas=",
        version = "v0.0.0-20140422174119-9fd32a8b3d3d",
    )
    go_repository(
        name = "com_github_bgentry_speakeasy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bgentry/speakeasy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ByYyxL9InA1OWqxJqqp2A5pYHUrCiAL6K3J+LKSsQkY=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_biogo_store",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/biogo/store",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tYoz1OeRpx3dJZlh9T4dQt4kAndcmpl+VNdzbSgFC/0=",
        version = "v0.0.0-20160505134755-913427a1d5e8",
    )
    go_repository(
        name = "com_github_bitly_go_simplejson",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bitly/go-simplejson",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6IH+V8/tVMab511d5bn4M7EwGXZf9Hj6i2xSwkNEM+Y=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_bits_and_blooms_bitset",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bits-and-blooms/bitset",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Kn4yilvwNtMACtf1eYDlG8H77R07mZSPbMjLyS07ChA=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_bkaradzic_go_lz4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bkaradzic/go-lz4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RXc4wYsyz985CkXXeX04y4VnZFGG8Rd43pRaHsOXAKk=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_bketelsen_crypt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bketelsen/crypt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:w/jqZtC9YD4DS/Vp9GhWfWcCpuAL58oTnLoI8vE9YHU=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_blang_semver",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blang/semver",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cQNTCjp13qL8KC3Nbxr/y2Bqb63oX6wdnnjpJbkM4JQ=",
        version = "v3.5.1+incompatible",
    )
    go_repository(
        name = "com_github_blevesearch_snowballstem",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/blevesearch/snowballstem",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lMQ189YspGP6sXvZQ4WZ+MLawfV8wOmPoD/iWeNXm8s=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_bmizerany_assert",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bmizerany/assert",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DDGfHa7BWjL4YnC6+E63dPcxHo2sUxDIu8g3QgEJdRY=",
        version = "v0.0.0-20160611221934-b7ed37b82869",
    )
    go_repository(
        name = "com_github_bmizerany_pat",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bmizerany/pat",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:y4B3+GPxKlrigF1ha5FFErxK+sr6sWxQovRMzwMhejo=",
        version = "v0.0.0-20170815010413-6226ea591a40",
    )
    go_repository(
        name = "com_github_boltdb_bolt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/boltdb/bolt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JQmyP4ZBrce+ZQu0dY660FMfatumYDLun9hBCUVIkF4=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_bonitoo_io_go_sql_bigquery",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bonitoo-io/go-sql-bigquery",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MaVh0h9+KaMnJcoDvvIGp+O3fefdWm+8MBUX6ELTJTM=",
        version = "v0.3.4-1.4.0",
    )
    go_repository(
        name = "com_github_bradleyfalzon_ghinstallation_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bradleyfalzon/ghinstallation/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ywF/8q+GVpvlsEuvRb1SGSDQDUxntW1d4kFu/9q/YAE=",
        version = "v2.0.3",
    )
    go_repository(
        name = "com_github_broady_gogeohash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/broady/gogeohash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iEdmkrNMLXbM7ecffOAtZJQOQUTE4iMonxrb5opUgE4=",
        version = "v0.0.0-20120525094510-7b2c40d64042",
    )
    go_repository(
        name = "com_github_bshuster_repo_logrus_logstash_hook",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bshuster-repo/logrus-logstash-hook",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pgAtgj+A31JBVtEHu2uHuEx0n+2ukqUJnS2vVe5pQNA=",
        version = "v0.4.1",
    )
    go_repository(
        name = "com_github_buchgr_bazel_remote",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/buchgr/bazel-remote",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_buchgr_bazel_remote.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6CLT+/PphNRuGL9KZ6LESNvoNg0lEv3zoVkq/i4uMpI=",
        version = "v1.3.3",
    )
    go_repository(
        name = "com_github_bufbuild_buf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bufbuild/buf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oBx0OPMc62HE73dXLQWu6AH+WzA0h0p8/TyA1JeVhks=",
        version = "v0.56.0",
    )
    go_repository(
        name = "com_github_buger_jsonparser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/buger/jsonparser",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+qUNY4VRkEH46bLUwxCyUU+iOGJMQBVibAaYzWiwWcg=",
        version = "v0.0.0-20200322175846-f7e751efca13",
    )
    go_repository(
        name = "com_github_bugsnag_bugsnag_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bugsnag/bugsnag-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rFt+Y/IK1aEZkEHchZRSq9OQbsSzIT/OrI8YFFmRIng=",
        version = "v0.0.0-20141110184014-b1d153021fcd",
    )
    go_repository(
        name = "com_github_bugsnag_osext",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bugsnag/osext",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:otBG+dV+YK+Soembjv71DPz3uX/V/6MMlSyD9JBQ6kQ=",
        version = "v0.0.0-20130617224835-0dd3f918b21b",
    )
    go_repository(
        name = "com_github_bugsnag_panicwrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/bugsnag/panicwrap",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nvj0OLI3YqYXer/kZD8Ri1aaunCxIEsOst1BVJswV0o=",
        version = "v0.0.0-20151223152923-e2c28503fcd0",
    )
    go_repository(
        name = "com_github_burntsushi_toml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/BurntSushi/toml",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9F2/+DoOYIOksmaJFPw1tGFy1eDnIJXg+UHjuD8lTak=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_burntsushi_xgb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/BurntSushi/xgb",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1BDTz0u9nC3//pOCMdNH+CiXJVYJh5UQNCOBG7jbELc=",
        version = "v0.0.0-20160522181843-27f122750802",
    )
    go_repository(
        name = "com_github_burntsushi_xgbutil",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/BurntSushi/xgbutil",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4ZrkT/RzpnROylmoQL57iVUL57wGKTR5O6KpVnbm2tA=",
        version = "v0.0.0-20160919175755-f7c97cef3b4e",
    )
    go_repository(
        name = "com_github_c_bata_go_prompt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/c-bata/go-prompt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uyKRz6Z6DUyj49QVijyM339UJV9yhbr70gESwbNU3e0=",
        version = "v0.2.2",
    )
    go_repository(
        name = "com_github_cactus_go_statsd_client_statsd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cactus/go-statsd-client/statsd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bXxS5/Z3/dfc8iFniQfgogNBomo0u+1//9eP+jl8GVo=",
        version = "v0.0.0-20191106001114-12b4e2b38748",
    )
    go_repository(
        name = "com_github_casbin_casbin_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/casbin/casbin/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bTwon/ECRx9dwBy2ewRVr5OiqjeXSGiTUY74sDPQi/g=",
        version = "v2.1.2",
    )
    go_repository(
        name = "com_github_cenkalti_backoff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cenkalti/backoff",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tNowT99t7UNflLxfYYSlKYsBpXdEet03Pg2g16Swow4=",
        version = "v2.2.1+incompatible",
    )
    go_repository(
        name = "com_github_cenkalti_backoff_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cenkalti/backoff/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cFAlzYUlVYDysBEH2T5hyJZMh3+5+WCBvSnK6Q8UtC4=",
        version = "v4.1.3",
    )
    go_repository(
        name = "com_github_census_instrumentation_opencensus_proto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/census-instrumentation/opencensus-proto",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:glEXhBS5PSLLv4IXzLA5yPRVX4bilULVyxxbrfOtDAk=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_cespare_xxhash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cespare/xxhash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:a6HrQnmkObjyL+Gs60czilIUGqrzKutQD6XZog3p+ko=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_cespare_xxhash_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cespare/xxhash/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UL815xU9SqsFlibzuggzjXhog7bL6oX9BbNZnL2UFvs=",
        version = "v2.3.0",
    )
    go_repository(
        name = "com_github_charmbracelet_bubbles",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/charmbracelet/bubbles",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OYmHuDqyuzWwiurw7eeYJ2482BENoX3hScpzamwF5K8=",
        version = "v0.15.1-0.20230123181021-a6a12c4a31eb",
    )
    go_repository(
        name = "com_github_charmbracelet_bubbletea",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/charmbracelet/bubbletea",
        replace = "github.com/cockroachdb/bubbletea",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OmQDBlTg1DU5OaKjIuEBt4K+40i9RqtxAwK1P/0OVjI=",
        version = "v0.23.1-bracketed-paste2",
    )
    go_repository(
        name = "com_github_charmbracelet_harmonica",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/charmbracelet/harmonica",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8NxJWRWg/bzKqqEaaeFNipOu77YR5t8aSwG4pgaUBiQ=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_charmbracelet_lipgloss",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/charmbracelet/lipgloss",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1StyZB9vBSOyuZxQUcUwGr17JmojPNm87inij9N3wJY=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_checkpoint_restore_go_criu_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/checkpoint-restore/go-criu/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WW2B2uxx9KWF6bGlHqhm8Okiafwwx7Y2kcpn8lCpjgo=",
        version = "v4.1.0",
    )
    go_repository(
        name = "com_github_checkpoint_restore_go_criu_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/checkpoint-restore/go-criu/v5",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wpFFOoomK3389ue2lAb0Boag6XPht5QYpipxmSNL4d8=",
        version = "v5.3.0",
    )
    go_repository(
        name = "com_github_chzyer_logex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chzyer/logex",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Swpa1K6QvQznwJRcfTfQJmTE72DqScAa40E+fbHEXEE=",
        version = "v1.1.10",
    )
    go_repository(
        name = "com_github_chzyer_readline",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chzyer/readline",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fY5BOSpyZCqRo5OhCuC+XN+r/bBCmeuuJtjz+bCNIf8=",
        version = "v0.0.0-20180603132655-2972be24d48e",
    )
    go_repository(
        name = "com_github_chzyer_test",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/chzyer/test",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:q763qf9huN11kDQavWsoZXJNW3xEE4JJyHa5Q25/sd8=",
        version = "v0.0.0-20180213035817-a1ea475d72b1",
    )
    go_repository(
        name = "com_github_cilium_ebpf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cilium/ebpf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1k/q3ATgxSXRdrmPfH8d7YK0GfqVsEKZAX9dQZvs56k=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_circonus_labs_circonus_gometrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/circonus-labs/circonus-gometrics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:C29Ae4G5GtYyYMm1aztcyj/J5ckgJm2zwdDajFbx1NY=",
        version = "v2.3.1+incompatible",
    )
    go_repository(
        name = "com_github_circonus_labs_circonusllhist",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/circonus-labs/circonusllhist",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TJH+oke8D16535+jHExHj4nQvzlZrj7ug5D7I/orNUA=",
        version = "v0.1.3",
    )
    go_repository(
        name = "com_github_clbanning_x2j",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/clbanning/x2j",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EdRZT3IeKQmfCSrgo8SZ8V3MEnskuJP0wCYNpe+aiXo=",
        version = "v0.0.0-20191024224557-825249438eec",
    )
    go_repository(
        name = "com_github_clickhouse_clickhouse_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ClickHouse/clickhouse-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cKjXeYLNWVJIx2J1K6H2CqyRmfwVJVY1OV1coaaFcI0=",
        version = "v1.5.4",
    )
    go_repository(
        name = "com_github_client9_misspell",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/client9/misspell",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ta993UF76GwbvJcIo3Y68y/M3WxlpEHPWIGDkJYwzJI=",
        version = "v0.3.4",
    )
    go_repository(
        name = "com_github_cloudflare_golz4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cloudflare/golz4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:F1EaeKL/ta07PY/k9Os/UFtwERei2/XzGemhpGnBKNg=",
        version = "v0.0.0-20150217214814-ef862a3cdc58",
    )
    go_repository(
        name = "com_github_cloudykit_fastprinter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/CloudyKit/fastprinter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sR+/8Yb4slttB4vD+b9btVEnWgL3Q00OBTzVT8B9C0c=",
        version = "v0.0.0-20200109182630-33d98a066a53",
    )
    go_repository(
        name = "com_github_cloudykit_jet_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/CloudyKit/jet/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1PwO5w5VCtlUUl+KTOBsTGZlhjWkcybsGaAau52tOy8=",
        version = "v3.0.0",
    )
    go_repository(
        name = "com_github_cloudykit_jet_v6",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/CloudyKit/jet/v6",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EpcZ6SR9n28BUGtNJSvlBqf90IpjeFr36Tizxhn/oME=",
        version = "v6.2.0",
    )
    go_repository(
        name = "com_github_cncf_udpa_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cncf/udpa/go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cqQfy1jclcSy/FwLjemeg3SR1yaINm74aQyupQ0Bl8M=",
        version = "v0.0.0-20201120205902-5459f2c99403",
    )
    go_repository(
        name = "com_github_cncf_xds_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cncf/xds/go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6xNmx7iTtyBRev0+D/Tv1FZd4SCg8axKApyNyRsAt/w=",
        version = "v0.0.0-20251210132809-ee656c7534f5",
    )
    go_repository(
        name = "com_github_cockroachdb_apd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/apd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3LFP3629v+1aKXU5Q37mxmRxX/pIu1nijXydLShEq5I=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_cockroachdb_apd_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/apd/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:U+8j7t0axsIgvQUqthuNm82HIrYXodOV2iWLWtEaIwg=",
        version = "v3.2.1",
    )
    go_repository(
        name = "com_github_cockroachdb_circuitbreaker",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/circuitbreaker",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u3uQ4oAKM5g2eODBAsDdDSrTs7zRWXtvu+nvSDA9098=",
        version = "v2.2.2-0.20190114160014-a614b14ccf63+incompatible",
    )
    go_repository(
        name = "com_github_cockroachdb_cmux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/cmux",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dzj1/xcivGjNPwwifh/dWTczkwcuqsXXFHY1X/TZMtw=",
        version = "v0.0.0-20170110192607-30d10be49292",
    )
    go_repository(
        name = "com_github_cockroachdb_cockroach_go_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/cockroach-go/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fNmtG6XhoA1DhdDCIu66YyGSsNb1szj4CaAsbDxRmy4=",
        version = "v2.3.3",
    )
    go_repository(
        name = "com_github_cockroachdb_crlfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/crlfmt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i0bnjanlWAvM50wHMT7EFyxlt5HQusznWrkwl+HBIsU=",
        version = "v0.0.0-20221214225007-b2fc5c302548",
    )
    go_repository(
        name = "com_github_cockroachdb_datadriven",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/datadriven",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1MLK4YpFtIEo3ZtMA5C795Wtv5VuUnrXX7mQG+aHg6o=",
        version = "v1.0.3-0.20230801171734-e384cf455877",
    )
    go_repository(
        name = "com_github_cockroachdb_errors",
        build_directives = [
            "gazelle:resolve proto proto gogoproto/gogo.proto @com_github_gogo_protobuf//gogoproto:gogo_proto",
            "gazelle:resolve proto go gogoproto/gogo.proto @com_github_gogo_protobuf//gogoproto",
            "gazelle:go_proto_compilers @com_github_cockroachdb_cockroach//pkg/cmd/protoc-gen-gogoroach:protoc-gen-gogoroach_compiler",
            "gazelle:go_grpc_compilers @com_github_cockroachdb_cockroach//pkg/cmd/protoc-gen-gogoroach:protoc-gen-gogoroach_grpc_compiler",
        ],
        build_file_proto_mode = "default",
        importpath = "github.com/cockroachdb/errors",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_cockroachdb_errors.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5bA+k2Y6r+oz/6Z/RFlNeVCesGARKuC6YymtcDrbC/I=",
        version = "v1.11.3",
    )
    go_repository(
        name = "com_github_cockroachdb_fifo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/fifo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:giXvy4KSc/6g/esnpM7Geqxka4WSqI1SZc7sMJFd3y4=",
        version = "v0.0.0-20240606204812-0bbfbd93a7ce",
    )
    go_repository(
        name = "com_github_cockroachdb_go_test_teamcity",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/go-test-teamcity",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YqzBA7tf8Gv8Oz0BbBsPenqkyjiohS7EUIwi7p1QJCU=",
        version = "v0.0.0-20191211140407-cff980ad0a55",
    )
    go_repository(
        name = "com_github_cockroachdb_gostdlib",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/gostdlib",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cSISxkVnTlWhTkyple/T6NXzOi5659FkhxvUgZv+Eb0=",
        version = "v1.19.0",
    )
    go_repository(
        name = "com_github_cockroachdb_logtags",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/logtags",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:r6VH0faHjZeQy818SGhaone5OnYfxFR/+AzdY3sf5aE=",
        version = "v0.0.0-20230118201751-21c54148d20b",
    )
    go_repository(
        name = "com_github_cockroachdb_pebble",
        build_directives = [
            "gazelle:build_tags invariants",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/pebble",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_cockroachdb_pebble.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5AAWCBWbat0uE0blr8qzufZP5tBjkRyy/jWe1QWLnvw=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_cockroachdb_redact",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/redact",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u1PMllDkdFfPWaNGMyLD1+so+aq3uUItthCFqzwPJ30=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_github_cockroachdb_returncheck",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/returncheck",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KFOt5I9nEKZgCnOSmy8r4Oykh8BYQO8bFOTgHDS8YZA=",
        version = "v0.0.0-20200612231554-92cdbca611dd",
    )
    go_repository(
        name = "com_github_cockroachdb_stress",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/stress",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JbkTu68NpiczWOaciC2QSy33RFEnnq6H2pz7viQp0rc=",
        version = "v0.0.0-20220803192808-1806698b1b7b",
    )
    go_repository(
        name = "com_github_cockroachdb_tokenbucket",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/tokenbucket",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zuQyyAKVxetITBuuhv3BI9cMrmStnpT18zmgmTxunpo=",
        version = "v0.0.0-20230807174530-cc333fc44b06",
    )
    go_repository(
        name = "com_github_cockroachdb_tools",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/tools",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qVTb3XEv+7VVjPetop8yGbN95HfCeqZx+VMveeSJPZw=",
        version = "v0.0.0-20211112185054-642e51449b40",
    )
    go_repository(
        name = "com_github_cockroachdb_ttycolor",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cockroachdb/ttycolor",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Hli+oX84dKq44sLVCcsGKqifm5Lg9J8VoJ2P3h9iPdI=",
        version = "v0.0.0-20210902133924-c7d7dcdde4e8",
    )
    go_repository(
        name = "com_github_codahale_hdrhistogram",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/codahale/hdrhistogram",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qMd81Ts1T2OTKmB4acZcyKaMtRnY5Y44NuXGX2GFJ1w=",
        version = "v0.0.0-20161010025455-3a0bb77429bd",
    )
    go_repository(
        name = "com_github_codefor_geohash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Codefor/geohash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iG9B49Q218F/XxXNRM7k/vWf7MKmLIS8AcJV9cGN4nA=",
        version = "v0.0.0-20140723084247-1b41c28e3a9d",
    )
    go_repository(
        name = "com_github_codegangsta_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/codegangsta/cli",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iX1FXEgwzd5+XN6wk5cVHOGQj6Q3Dcp20lUeS4lHNTw=",
        version = "v1.20.0",
    )
    go_repository(
        name = "com_github_codegangsta_inject",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/codegangsta/inject",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sDMmm+q/3+BukdIpxwO365v/Rbspp2Nt5XntgQRXq8Q=",
        version = "v0.0.0-20150114235600-33e0aa1cb7c0",
    )
    go_repository(
        name = "com_github_containerd_aufs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/aufs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2oeJiwX5HstO7shSrPZjrohJZLzK36wvpdmzDRkL/LY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_containerd_btrfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/btrfs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:osn1exbzdub9L5SouXO5swW4ea/xVdJZ3wokxN5GrnA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_containerd_cgroups",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/cgroups",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iJnMvco9XGvKUvNQkv88bE4uJXxRQH18efbKo9w5vHQ=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_containerd_console",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/console",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lIr7SlA5PxZyMV30bDW0MGbiOPXwc63yRuCP0ARubLw=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_containerd_containerd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/containerd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uPF0og3ByFzDnaStfiQj3fVGTEtaSNyU+bW7GR/nqGA=",
        version = "v1.5.4",
    )
    go_repository(
        name = "com_github_containerd_continuity",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/continuity",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nisirsYROK15TAMVukJOUyGJjz4BNQJBVsNvAXZJ/eg=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_containerd_fifo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/fifo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6PirWBr9/L7GDamKr+XM0IeUFXu5mf3M/BPpH9gaLBU=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_containerd_go_cni",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/go-cni",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YbJAhpTevL2v6u8JC1NhCYRwf+3Vzxcc5vGnYoJ7VeE=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_containerd_go_runc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/go-runc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oU+lLv1ULm5taqgV/CJivypVODI4SUz1znWjv3nNYS0=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_containerd_imgcrypt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/imgcrypt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LBwiTfoUsdiEGAR1TpvxE+Gzt7469oVu87iR3mv3Byc=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_containerd_nri",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/nri",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6QioHRlThlKh2RkRTR4kIT3PKAcrLo3gIWnjkM4dQmQ=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_containerd_ttrpc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/ttrpc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2/O3oTZN36q2xRolk0a2WWGgh7/Vf/liElg5hFYLX9U=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_containerd_typeurl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/typeurl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Chlt8zIieDbzQFzXzAeBEF92KhExuE4p9p92/QmY7aY=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_containerd_zfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containerd/zfs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cXLJbx+4Jj7rNsTiqVfm6i+RNLx6FFA2fMmDlEf+Wm8=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_containernetworking_cni",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containernetworking/cni",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7zpDnQ3T3s4ucOuJ/ZCLrYBxzkg0AELFfII3Epo9TmI=",
        version = "v0.8.1",
    )
    go_repository(
        name = "com_github_containernetworking_plugins",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containernetworking/plugins",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FD1tADPls2EEi3flPc2OegIY1M9pUa9r2Quag7HMLV8=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_containers_ocicrypt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/containers/ocicrypt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:prL8l9w3ntVqXvNH1CiNn5ENjcCnr38JqpSyvKKB4GI=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_coreos_bbolt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/bbolt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wZwiHHUieZCquLkDL0B8UhzreNWsPHooDAG3q34zk0s=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_coreos_etcd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/etcd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8F3hqu9fGYLBifCmRCJsicFqDx/D68Rt3q1JMazcgBQ=",
        version = "v3.3.13+incompatible",
    )
    go_repository(
        name = "com_github_coreos_go_etcd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-etcd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bXhRBIXoTm9BYHS3gE0TtQuyNZyeEMux2sDi4oo5YOo=",
        version = "v2.0.0+incompatible",
    )
    go_repository(
        name = "com_github_coreos_go_iptables",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-iptables",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mw6SAibtHKZcNzAsOxjoHIG0gy5YFHhypWSSNc6EjbQ=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_coreos_go_oidc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-oidc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mh48q/BqXqgjVHpy2ZY7WnWAbenxRjsz9N1i1YxjHAk=",
        version = "v2.2.1+incompatible",
    )
    go_repository(
        name = "com_github_coreos_go_semver",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-semver",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wkHLiw0WNATZnSG7epLsujiMCgPAc9xhjJ4tgnAxmfM=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_coreos_go_systemd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-systemd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JOrtw2xFKzlg+cbHpyrpLDmnN1HqhBfnX7WDiW7eG2c=",
        version = "v0.0.0-20190719114852-fd7a80b32e1f",
    )
    go_repository(
        name = "com_github_coreos_go_systemd_v22",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/go-systemd/v22",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:D9/bQk5vlXQFZ6Kwuu6zaiXJ9oTPe68++AzAJc1DzSI=",
        version = "v22.3.2",
    )
    go_repository(
        name = "com_github_coreos_pkg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/coreos/pkg",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lBNOc5arjvs8E5mO2tbpBpLoyyu8B6e44T7hJy6potg=",
        version = "v0.0.0-20180928190104-399ea9e2e55f",
    )
    go_repository(
        name = "com_github_corpix_uarand",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/corpix/uarand",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RMr1TWc9F4n5jiPDzFHtmaUXLKLNUFK0SgCLo4BhX/U=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_cpuguy83_go_md2man",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cpuguy83/go-md2man",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BSKMNlYxDvnunlTymqtgONjNnaRV1sTpcovwwjF22jk=",
        version = "v1.0.10",
    )
    go_repository(
        name = "com_github_cpuguy83_go_md2man_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cpuguy83/go-md2man/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:r/myEWzV9lfsM1tFLgDyu0atFtJ1fXn261LKYj/3DxU=",
        version = "v2.0.1",
    )
    go_repository(
        name = "com_github_creack_pty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/creack/pty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:07n33Z8lZxZ2qwegKbObQohDhXDQxiMMz1NOUGYlesw=",
        version = "v1.1.11",
    )
    go_repository(
        name = "com_github_cyberdelia_go_metrics_graphite",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cyberdelia/go-metrics-graphite",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:M5QgkYacWj0Xs8MhpIK/5uwU02icXpEoSo9sM2aRCps=",
        version = "v0.0.0-20161219230853-39f87cc3b432",
    )
    go_repository(
        name = "com_github_cyberdelia_templates",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cyberdelia/templates",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/ovYnF02fwL0kvspmy9AuyKg1JhdTRUgPw4nUxd9oZM=",
        version = "v0.0.0-20141128023046-ca7fffd4298c",
    )
    go_repository(
        name = "com_github_cyphar_filepath_securejoin",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/cyphar/filepath-securejoin",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YX6ebbZCZP7VkM3scTTokDgBL2TY741X51MTk3ycuNI=",
        version = "v0.2.3",
    )
    go_repository(
        name = "com_github_d2g_dhcp4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/d2g/dhcp4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Xo2rK1pzOm0jO6abTPIQwbAmqBIOj132otexc1mmzFc=",
        version = "v0.0.0-20170904100407-a1d1b6c41b1c",
    )
    go_repository(
        name = "com_github_d2g_dhcp4client",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/d2g/dhcp4client",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:suYBsYZIkSlUMEz4TAYCczKf62IA2UWC+O8+KtdOhCo=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_d2g_dhcp4server",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/d2g/dhcp4server",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+CpLbZIeUn94m02LdEKPcgErLJ347NUwxPKs5u8ieiY=",
        version = "v0.0.0-20181031114812-7d4a0a7f59a5",
    )
    go_repository(
        name = "com_github_d2g_hardwareaddr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/d2g/hardwareaddr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:itqmmf1PFpC4n5JW+j4BU7X4MTfVurhYRTjODoPb2Y8=",
        version = "v0.0.0-20190221164911-e7d9fbe030e4",
    )
    go_repository(
        name = "com_github_daaku_go_zipexe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/daaku/go.zipexe",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VSOgZtH418pH9L16hC/JrgSNJbbAL26pj7lmD1+CGdY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_data_dog_go_sqlmock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/DATA-DOG/go-sqlmock",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Shsta01QNfFxHCfpW6YH2STWB0MudeXXEWMr20OEh60=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_datadog_datadog_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/DataDog/datadog-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qSG2N4FghB1He/r2mFrWKCaL7dXCilEuNEeAn20fdD4=",
        version = "v3.2.0+incompatible",
    )
    go_repository(
        name = "com_github_datadog_zstd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/DataDog/zstd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+K/VEwIAaPcHiMtQvpLD4lqW7f0Gk3xdYZmI1hD+CXo=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_dave_dst",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dave/dst",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5wtsjxee7nUDlKEz4i6ewJVn1193vfv2UpEXKqzmaUI=",
        version = "v0.24.0",
    )
    go_repository(
        name = "com_github_dave_gopackages",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dave/gopackages",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:l99YKCdrK4Lvb/zTupt0GMPfNbncAGf8Cv/t1sYLOg0=",
        version = "v0.0.0-20170318123100-46e7023ec56e",
    )
    go_repository(
        name = "com_github_dave_jennifer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dave/jennifer",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:S15ZkFMRoJ36mGAQgWL1tnr0NQJh9rZ8qatseX/VbBc=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_dave_kerr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dave/kerr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xURkGi4RydhyaYR6PzcyHTueQudxY4LgxN1oYEPJHa0=",
        version = "v0.0.0-20170318121727-bc25dd6abe8e",
    )
    go_repository(
        name = "com_github_dave_rebecca",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dave/rebecca",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jxVfdOxRirbXL28vXMvUvJ1in3djwkVKXCq339qhBL0=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_davecgh_go_spew",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/davecgh/go-spew",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:U9qPSI2PIWSS1VwoXQT9A3Wy9MM3WgvqSxFWenqJduM=",
        version = "v1.1.2-0.20180830191138-d8f796af33cc",
    )
    go_repository(
        name = "com_github_decred_dcrd_crypto_blake256",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/decred/dcrd/crypto/blake256",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/8DMNYp9SGi5f0w7uCm6d6M4OU2rGFK09Y2A4Xv7EE0=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_decred_dcrd_dcrec_secp256k1_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/decred/dcrd/dcrec/secp256k1/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HbphB4TFFXpv7MNrT52FGrrgVXF1owhMVTHFZIlnvd4=",
        version = "v4.1.0",
    )
    go_repository(
        name = "com_github_deepmap_oapi_codegen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/deepmap/oapi-codegen",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:w/d1ntwh91XI0b/8ja7+u5SvA4IFfM0UNNLmiDR1gg0=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_denisenkom_go_mssqldb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/denisenkom/go-mssqldb",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VtrkII767ttSPNRfFekePK3sctr+joXgO58stqQbtUA=",
        version = "v0.12.0",
    )
    go_repository(
        name = "com_github_dennwc_varint",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dennwc/varint",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kGNFFSSw8ToIy3obO/kKr8U9GZYUAxQEVuix4zfDWzE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_denverdino_aliyungo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/denverdino/aliyungo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:p6poVbjHDkKa+wtC8frBMwQtT3BmqGYBjzMwJ63tuR4=",
        version = "v0.0.0-20190125010748-a747050bb1ba",
    )
    go_repository(
        name = "com_github_dgraph_io_badger",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgraph-io/badger",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DshxFxZWXUcO0xX476VJC07Xsr6ZCBVRHKZ93Oh7Evo=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_dgrijalva_jwt_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgrijalva/jwt-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7qlOGliEKZXTDg6OTjfoBKDXWrumCAMpl/TFQ4/5kLM=",
        version = "v3.2.0+incompatible",
    )
    go_repository(
        name = "com_github_dgrijalva_jwt_go_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgrijalva/jwt-go/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CaO/zOnF8VvUfEbhRatPcwKVWamvbYd8tQGRWacE9kU=",
        version = "v4.0.0-preview1",
    )
    go_repository(
        name = "com_github_dgryski_go_bitstream",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/go-bitstream",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:akOQj8IVgoeFfBTzGOEQakCYshWD6RNo1M5pivFXt70=",
        version = "v0.0.0-20180413035011-3522498ce2c8",
    )
    go_repository(
        name = "com_github_dgryski_go_farm",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/go-farm",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tdlZCpZ/P9DhczCTSixgIKmwPv6+wP5DGjqLYw5SUiA=",
        version = "v0.0.0-20190423205320-6a90982ecee2",
    )
    go_repository(
        name = "com_github_dgryski_go_metro",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/go-metro",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8WFBn63wegobsYAX0YjD+8suexZDga5CctH4CCTx2+8=",
        version = "v0.0.0-20180109044635-280f6062b5bc",
    )
    go_repository(
        name = "com_github_dgryski_go_sip13",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dgryski/go-sip13",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9cOfvEwjQxdwKuNDTQSaMKNRvwKwgZG+U4HrjeRKHso=",
        version = "v0.0.0-20200911182023-62edffca9245",
    )
    go_repository(
        name = "com_github_digitalocean_godo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/digitalocean/godo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3SywGJBC18HaYtPQF+T36jYzXBi+a6eIMonSjDll7TA=",
        version = "v1.65.0",
    )
    go_repository(
        name = "com_github_dimchansky_utfbom",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dimchansky/utfbom",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vV6w1AhK4VMnhBno/TPVCoK9U/LP0PkLCS9tbxHdi/U=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_djherbis_atime",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/djherbis/atime",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rgwVbP/5by8BvvjBNrbh64Qz33idKT3pSnMSJsxhi0g=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_dnaeon_go_vcr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dnaeon/go-vcr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zHCHvJYTMh1N7xnV7zf1m1GPBF9Ad0Jk/whtQ1663qI=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_docker_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/cli",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:eO2KS7ZFeov5UJeaDmIs1NFEDRf32PaqRpvoEkKBy5M=",
        version = "v20.10.17+incompatible",
    )
    go_repository(
        name = "com_github_docker_distribution",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/distribution",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:a5mlkVzth6W5A4fOsS3D2EO5BUmsJpcB+cRlLU7cSug=",
        version = "v2.7.1+incompatible",
    )
    go_repository(
        name = "com_github_docker_docker",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/docker",
        replace = "github.com/moby/moby",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3wn5wW3KwjAv8Z36VHdbvaqvY273JiWUDFuudH0z5Vs=",
        version = "v20.10.6+incompatible",
    )
    go_repository(
        name = "com_github_docker_go_connections",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-connections",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:El9xVISelRB7BuFusrZozjnkIM5YnzCViNKohAFqRJQ=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_docker_go_events",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-events",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+pKlWGMw7gf6bQ+oDZB4KHQFypsfjYlq/C4rfL7D3g8=",
        version = "v0.0.0-20190806004212-e31b211e4f1c",
    )
    go_repository(
        name = "com_github_docker_go_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-metrics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AgB/0SvBxihN0X8OR4SjsblXkbMvalQ8cjmtKQ2rQV8=",
        version = "v0.0.1",
    )
    go_repository(
        name = "com_github_docker_go_units",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/go-units",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3uh0PgVws3nIA0Q+MwDC8yjEPf9zjRfZZWXZYDct3Tw=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_docker_libtrust",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/libtrust",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ZClxb8laGDf5arXfYcAtECDFgAgHklGI8CxgjHnXKJ4=",
        version = "v0.0.0-20150114040149-fa567046d9b1",
    )
    go_repository(
        name = "com_github_docker_spdystream",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docker/spdystream",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cenwrSVm+Z7QLSV/BsnenAOcDXdX4cMv4wP0B/5QbPg=",
        version = "v0.0.0-20160310174837-449fdfce4d96",
    )
    go_repository(
        name = "com_github_docopt_docopt_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/docopt/docopt-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bWDMxwH3px2JBh6AyO7hdCn/PkvCZXii8TGj7sbtEbQ=",
        version = "v0.0.0-20180111231733-ee0de3bc6815",
    )
    go_repository(
        name = "com_github_dustin_go_humanize",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dustin/go-humanize",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VSnTsYCnlFHaM2/igO1h6X3HA71jcobQuxemgkq4zYo=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_dvyukov_go_fuzz",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/dvyukov/go-fuzz",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:q1oJaUPdmpDm/VyXosjgPgr6wS7c5iV2p0PwJD73bUI=",
        version = "v0.0.0-20210103155950-6a8e9d1f2415",
    )
    go_repository(
        name = "com_github_eapache_go_resiliency",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/eapache/go-resiliency",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RRL0nge+cWGlxXbUzJ7yMcq6w2XBEr19dCN6HECGaT0=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_eapache_go_xerial_snappy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/eapache/go-xerial-snappy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8yY/I9ndfrgrXUbOGObLHKBR4Fl3nZXwM2c7OYTT8hM=",
        version = "v0.0.0-20230111030713-bf00bc1b83b6",
    )
    go_repository(
        name = "com_github_eapache_queue",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/eapache/queue",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YOEu7KNc61ntiQlcEeUIoDTJ2o8mQznoNvUhiigpIqc=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_eclipse_paho_mqtt_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/eclipse/paho.mqtt.golang",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1F8mhG9+aO5/xpdtFkW4SxOJB67ukuDC3t2y2qayIX0=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_edsrzf_mmap_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/edsrzf/mmap-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CEBF7HpRnUCSJgGUb5h1Gm7e3VkmVDrR8lvWVLtrOFw=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_eknkc_amber",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/eknkc/amber",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:clC1lXBpe2kTj2VHdaIu9ajZQe4kcEY9j0NsnDDBZ3o=",
        version = "v0.0.0-20171010120322-cdade1c07385",
    )
    go_repository(
        name = "com_github_elastic_gosigar",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/elastic/gosigar",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:T0aQ7n/n2ZA9W7DmAnj60v+qzqKERdBgJBO1CG2W6rc=",
        version = "v0.14.1",
    )
    go_repository(
        name = "com_github_elazarl_go_bindata_assetfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/elazarl/go-bindata-assetfs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:m0kkaHRKEu7tUIUFVwhGGGYClXvyl4RE03qmvRTNfbw=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_elazarl_goproxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/elazarl/goproxy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yUdfgN0XgIJw7foRItutHYUIhlcKzcSf5vDpdhQAKTc=",
        version = "v0.0.0-20180725130230-947c36da3153",
    )
    go_repository(
        name = "com_github_emicklei_dot",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/emicklei/dot",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XDBW0Xco1QNyRb33cqLe10cT04yMWL1XpCZfa98Q6Og=",
        version = "v0.15.0",
    )
    go_repository(
        name = "com_github_emicklei_go_restful",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/emicklei/go-restful",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:SIvoTSbsMEwuM3dzFirLwKc4BH6VXP5CNf+G1FfJVr4=",
        version = "v2.12.0+incompatible",
    )
    go_repository(
        name = "com_github_emicklei_go_restful_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/emicklei/go-restful/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:R5iagkkTsfFpe1nce6Yniw3wmSx0Qw5YP469YzRFNEs=",
        version = "v3.7.1",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/envoyproxy/go-control-plane",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hbG2kr4RuFj222B6+7T83thSPqLjwBIfQawTkC++2HA=",
        version = "v0.14.0",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane_envoy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/envoyproxy/go-control-plane/envoy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yg/JjO5E7ubRyKX3m07GF3reDNEnfOboJ0QySbH736g=",
        version = "v1.36.0",
    )
    go_repository(
        name = "com_github_envoyproxy_go_control_plane_ratelimit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/envoyproxy/go-control-plane/ratelimit",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/G9QYbddjL25KvtKTv3an9lx6VBE2cnb8wp1vEGNYGI=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_envoyproxy_protoc_gen_validate",
        build_file_proto_mode = "disable_global",
        build_naming_convention = "go_default_library",
        importpath = "github.com/envoyproxy/protoc-gen-validate",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TvGH1wof4H33rezVKWSpqKz5NXWg5VPuZ0uONDT6eb4=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_etcd_io_bbolt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/etcd-io/bbolt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gSJmxrs37LgTqR/oyJBWok6k6SvXEUerFTbltIhXkBM=",
        version = "v1.3.3",
    )
    go_repository(
        name = "com_github_evanphx_json_patch",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/evanphx/json-patch",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:glyUF9yIYtMHzn8xaKw5rMhdWcwsYV8dZHIq5567/xs=",
        version = "v4.11.0+incompatible",
    )
    go_repository(
        name = "com_github_facebookgo_clock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/facebookgo/clock",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yDWHCSQ40h88yih2JAcL6Ls/kVkSE8GFACTGVnMPruw=",
        version = "v0.0.0-20150410010913-600d898af40a",
    )
    go_repository(
        name = "com_github_fanixk_geohash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fanixk/geohash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Fyfh/dsHFrC6nkX7H7+nFdTd1wROlX/FxEIWVpKYf1U=",
        version = "v0.0.0-20150324002647-c1f9b5fa157a",
    )
    go_repository(
        name = "com_github_fasthttp_contrib_websocket",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fasthttp-contrib/websocket",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DddqAaWDpywytcG8w/qoQ5sAN8X12d3Z3koB0C3Rxsc=",
        version = "v0.0.0-20160511215533-1f3b11f56072",
    )
    go_repository(
        name = "com_github_fasthttp_router",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fasthttp/router",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Z025tHFTjDp6T6QMBjloyGL6KV5wtakW365K/7KiE1c=",
        version = "v1.4.4",
    )
    go_repository(
        name = "com_github_fatih_color",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fatih/color",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8xPHl4/q1VyqGIPif1F+1V3Y3lSmrq01EabUW3CoW5s=",
        version = "v1.9.0",
    )
    go_repository(
        name = "com_github_fatih_structs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fatih/structs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Q7juDM0QtcnhCpeyLGQKyg4TOIghuNXrkL32pHAUMxo=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_felixge_httpsnoop",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/felixge/httpsnoop",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:NFTV2Zj1bL4mc9sqWACXbQFVBBg2W3GPvqp8/ESS2Wg=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_flosch_pongo2_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/flosch/pongo2/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gv+5Pe3vaSVmiJvh/BZa82b7/00YUGm0PIyVVLop0Hw=",
        version = "v4.0.2",
    )
    go_repository(
        name = "com_github_fogleman_gg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fogleman/gg",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WXb3TSNmHp2vHoCroCIB1foO/yQ36swABL8aOVeDpgg=",
        version = "v1.2.1-0.20190220221249-0403632d5b90",
    )
    go_repository(
        name = "com_github_form3tech_oss_jwt_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/form3tech-oss/jwt-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7ZaBxOI7TMoYBfyA3cQHErNNyAWIKUMIwqxEtgHOs5c=",
        version = "v3.2.3+incompatible",
    )
    go_repository(
        name = "com_github_fortytw2_leaktest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fortytw2/leaktest",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u8491cBMTQ8ft8aeV+adlcytMZylmA5nnwwkRZjI8vw=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_foxcpp_go_mockdns",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/foxcpp/go-mockdns",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nLPjjvpUAODOR6vY/7o0hBIk8iTr19Fvmf8aFx/kC7A=",
        version = "v0.0.0-20201212160233-ede2f9158d15",
    )
    go_repository(
        name = "com_github_franela_goblin",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/franela/goblin",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gb2Z18BhTPJPpLQWj4T+rfKHYCHxRHCtRxhKKjRidVw=",
        version = "v0.0.0-20200105215937-c9ffbefa60db",
    )
    go_repository(
        name = "com_github_franela_goreq",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/franela/goreq",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:a9ENSRDFBUPkJ5lCgVZh26+ZbGyoVJG7yb5SSzF5H54=",
        version = "v0.0.0-20171204163338-bcd34c9993f8",
    )
    go_repository(
        name = "com_github_frankban_quicktest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/frankban/quicktest",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8sXhOn0uLys67V8EsXLc6eszDs8VXWxL3iRvebPhedY=",
        version = "v1.11.3",
    )
    go_repository(
        name = "com_github_fraugster_parquet_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fraugster/parquet-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:whX91AO3dkkOnbH9MqD53DZ3rISw+Tnnj5yiqXjSv9Q=",
        version = "v0.10.0",
    )
    go_repository(
        name = "com_github_fsnotify_fsnotify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fsnotify/fsnotify",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mZcQUHVQUQWoPXXtuf9yuEXKudkV2sx1E06UadKWpgI=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_fullsailor_pkcs7",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/fullsailor/pkcs7",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RDBNVkRviHZtvDvId8XSGPu3rmpmSe+wKRcEWNgsfWU=",
        version = "v0.0.0-20190404230743-d7302db945fa",
    )
    go_repository(
        name = "com_github_garyburd_redigo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/garyburd/redigo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LofdAjjjqCSXMwLGgOgnE+rdPuvX9DxCqaHwKy7i/ko=",
        version = "v0.0.0-20150301180006-535138d7bcd7",
    )
    go_repository(
        name = "com_github_gavv_httpexpect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gavv/httpexpect",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1X9kcRshkSKEjNJJxX9Y9mQ5BRfbxU5kORdjhlA1yX8=",
        version = "v2.0.0+incompatible",
    )
    go_repository(
        name = "com_github_geertjohan_go_incremental",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GeertJohan/go.incremental",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7AH+pY1XUgQE4Y1HcXYaMqAI0m9yrFqo/jt0CW30vsg=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_geertjohan_go_rice",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GeertJohan/go.rice",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KkI6O9uMaQU3VEKaj01ulavtF7o1fWT7+pk/4voiMLQ=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_getkin_kin_openapi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/getkin/kin-openapi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7WzP+MZRRe7YQz2Kc74Ley3dukJmXDvifVbElGmQfoA=",
        version = "v0.53.0",
    )
    go_repository(
        name = "com_github_getsentry_sentry_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/getsentry/sentry-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Pv98CIbtB3LkMWmXi4Joa5OOcwbmnX88sF5qbK3r3Ps=",
        version = "v0.27.0",
    )
    go_repository(
        name = "com_github_ghemawat_stream",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ghemawat/stream",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:r5GgOLGbza2wVHRzK7aAj6lWZjfbAwiu/RDCVOKjRyM=",
        version = "v0.0.0-20171120220530-696b145b53b9",
    )
    go_repository(
        name = "com_github_ghodss_yaml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ghodss/yaml",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wQHKEahhL6wmXdzwWG11gIVCkOv05bNOh+Rxn0yngAk=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_gin_contrib_sse",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gin-contrib/sse",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Y/yl/+YNO8GZSjAhjMsSuLt29uWRFHdHYUb5lYOV9qE=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_gin_gonic_gin",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gin-gonic/gin",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4+fr/el88TOO3ewCmQr8cx/CtZ/umlIRIs5M4NTNjf8=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_github_globalsign_mgo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/globalsign/mgo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DujepqpGd1hyOd7aW59XpK7Qymp8iy83xq74fLr21is=",
        version = "v0.0.0-20181015135952-eeefdecb41b8",
    )
    go_repository(
        name = "com_github_glycerine_go_unsnap_stream",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/glycerine/go-unsnap-stream",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:r04MMPyLHj/QwZuMJ5+7tJcBr1AQjpiAK/rZWRrQT7o=",
        version = "v0.0.0-20180323001048-9f0cb55181dd",
    )
    go_repository(
        name = "com_github_glycerine_goconvey",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/glycerine/goconvey",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gclg6gY70GLy3PbkQ1AERPfmLMMagS60DKF78eWwLn8=",
        version = "v0.0.0-20190410193231-58a59202ab31",
    )
    go_repository(
        name = "com_github_go_check_check",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-check/check",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0gkP6mzaMqkmpcJYCFOLkIBwI7xFExG03bbkOkCvUPI=",
        version = "v0.0.0-20180628173108-788fd7840127",
    )
    go_repository(
        name = "com_github_go_chi_chi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-chi/chi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fGFk2Gmi/YKXk0OmGfBh0WgmN3XB8lVnEyNz34tQRec=",
        version = "v4.1.2+incompatible",
    )
    go_repository(
        name = "com_github_go_chi_chi_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-chi/chi/v5",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DBPx88FjZJH3FsICfDAfIfnb7XxKIYVGG6lOPlhENAg=",
        version = "v5.0.0",
    )
    go_repository(
        name = "com_github_go_errors_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-errors/errors",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:J6MZopCL4uSllY1OfXM374weqZFFItUbrImctkmUxIA=",
        version = "v1.4.2",
    )
    go_repository(
        name = "com_github_go_fonts_liberation",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-fonts/liberation",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jAkAWJP4S+OsrPLZM4/eC9iW7CtHy+HBXrEwZXWo5VM=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_go_gl_glfw",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-gl/glfw",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QbL/5oDUmRBzO9/Z7Seo6zf912W/a6Sr4Eu0G/3Jho0=",
        version = "v0.0.0-20190409004039-e6da0acd62b1",
    )
    go_repository(
        name = "com_github_go_gl_glfw_v3_3_glfw",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-gl/glfw/v3.3/glfw",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WtGNWLvXpe6ZudgnXrq0barxBImvnnJoMEhXAzcbM0I=",
        version = "v0.0.0-20200222043503-6f7a984d4dc4",
    )
    go_repository(
        name = "com_github_go_ini_ini",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ini/ini",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Mujh4R/dH6YL8bxuISne3xX2+qcQ9p0IxKAP6ExWoUo=",
        version = "v1.25.4",
    )
    go_repository(
        name = "com_github_go_jose_go_jose_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-jose/go-jose/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CVLmWDhDVRa6Mi/IgCgaopNosCaHz7zrMeF9MlZRkrs=",
        version = "v4.1.3",
    )
    go_repository(
        name = "com_github_go_kit_kit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-kit/kit",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dXFJfIHVvUcpSgDOV+Ne6t7jXri8Tfv2uOLHUZ2XNuo=",
        version = "v0.10.0",
    )
    go_repository(
        name = "com_github_go_kit_log",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-kit/log",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MRVx0/zhvdseW+Gza6N9rVzU/IVzaeE1SFI4raAhmBU=",
        version = "v0.2.1",
    )
    go_repository(
        name = "com_github_go_latex_latex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-latex/latex",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6zl3BbBhdnMkpSj2YY30qV3gDcVBGtFgVsV3+/i+mKQ=",
        version = "v0.0.0-20210823091927-c0d11ff05a81",
    )
    go_repository(
        name = "com_github_go_logfmt_logfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-logfmt/logfmt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:otpy5pqBCBZ1ng9RQ0dPu4PN7ba75Y/aA+UpowDyNVA=",
        version = "v0.5.1",
    )
    go_repository(
        name = "com_github_go_logr_logr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-logr/logr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CjnDlHq8ikf6E492q6eKboGOC0T8CDaOvkHCIg8idEI=",
        version = "v1.4.3",
    )
    go_repository(
        name = "com_github_go_logr_stdr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-logr/stdr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hSWxHoqTgW2S2qGc0LTAI563KZ5YKYRhT3MFKZMbjag=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_go_martini_martini",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-martini/martini",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xveKWz2iaueeTaUgdetzel+U7exyigDYBryyVfV/rZk=",
        version = "v0.0.0-20170121215854-22fa46961aab",
    )
    go_repository(
        name = "com_github_go_ole_go_ole",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-ole/go-ole",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/Fpf6oFPoeFik9ty7siob0G6Ke8QvQEuVcuChpwXzpY=",
        version = "v1.2.6",
    )
    go_repository(
        name = "com_github_go_openapi_analysis",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/analysis",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UN09o0kNhleunxW7LR+KnltD0YrJ8FF03pSqvAN3Vro=",
        version = "v0.20.0",
    )
    go_repository(
        name = "com_github_go_openapi_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/errors",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Sxpo9PjEHDzhs3FbnGNonvDgWcMW2U7wGTcDDSFSceM=",
        version = "v0.20.0",
    )
    go_repository(
        name = "com_github_go_openapi_inflect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/inflect",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9jCH9scKIbHeV9m12SmPilScz6krDxKRasNNSNPXu/4=",
        version = "v0.19.0",
    )
    go_repository(
        name = "com_github_go_openapi_jsonpointer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/jsonpointer",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gZr+CIYByUqjcgeLXnQu2gHYQC9o73G2XUeOFYEICuY=",
        version = "v0.19.5",
    )
    go_repository(
        name = "com_github_go_openapi_jsonreference",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/jsonreference",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1WJP/wi4OjB4iV8KVbH73rQaoialJrqv8gitZLxGLtM=",
        version = "v0.19.5",
    )
    go_repository(
        name = "com_github_go_openapi_loads",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/loads",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:z5p5Xf5wujMxS1y8aP+vxwW5qYT2zdJBbXKmQUG3lcc=",
        version = "v0.20.2",
    )
    go_repository(
        name = "com_github_go_openapi_runtime",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/runtime",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5IIvCaIDbxetN674vX9eOxvoZ9mYGQ16fV1Q0VSG+NA=",
        version = "v0.19.29",
    )
    go_repository(
        name = "com_github_go_openapi_spec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/spec",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uH9RQ6vdyPSs2pSy9fL8QPspDF2AMIMPtmK5coSSjtQ=",
        version = "v0.20.3",
    )
    go_repository(
        name = "com_github_go_openapi_strfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/strfmt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6XZL+fF4VZYFxKQGLAUB358hOrRh/wS51uWEtlONADE=",
        version = "v0.20.2",
    )
    go_repository(
        name = "com_github_go_openapi_swag",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/swag",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:D2NRCBzS9/pEY3gP9Nl8aDqGUcPFrwG2p+CNFrLyrCM=",
        version = "v0.19.15",
    )
    go_repository(
        name = "com_github_go_openapi_validate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-openapi/validate",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AhqDegYV3J3iQkMPJSXkvzymHKMTw0BST3RK3hTT4ts=",
        version = "v0.20.2",
    )
    go_repository(
        name = "com_github_go_pdf_fpdf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-pdf/fpdf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MlgtGIfsdMEEQJr2le6b/HNr1ZlQwxyWr77r2aj2U/8=",
        version = "v0.6.0",
    )
    go_repository(
        name = "com_github_go_playground_assert_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-playground/assert/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MsBgLAaY856+nPRTKrp3/OZK38U/wa0CcBYNjji3q3A=",
        version = "v2.0.1",
    )
    go_repository(
        name = "com_github_go_playground_locales",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-playground/locales",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u50s323jtVGugKlcYeyzC0etD1HifMjqmJqb8WugfUU=",
        version = "v0.14.0",
    )
    go_repository(
        name = "com_github_go_playground_universal_translator",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-playground/universal-translator",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:82dyy6p4OuJq4/CByFNOn/jYrnRPArHwAcmLoJZxyho=",
        version = "v0.18.0",
    )
    go_repository(
        name = "com_github_go_playground_validator_v10",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-playground/validator/v10",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:prmOlTVv+YjZjmRmNSF3VmspqJIxJWXmqUsHwfTRRkQ=",
        version = "v10.11.1",
    )
    go_repository(
        name = "com_github_go_resty_resty_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-resty/resty/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JVrqSeQfdhYRFk24TvhTZWU0q8lfCojxZQFi3Ou7+uY=",
        version = "v2.1.1-0.20191201195748-d7b97669fe48",
    )
    go_repository(
        name = "com_github_go_sql_driver_mysql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-sql-driver/mysql",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BCTh4TKNUYmOmMUcQ3IipzF5prigylS7XXjEkfCHuOE=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_go_stack_stack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-stack/stack",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5SgMzNM5HxrEjV0ww2lTmX6E2Izsfxas4+YHWRs3Lsk=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_go_swagger_go_swagger",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-swagger/go-swagger",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1XUWLnH6hKxHzeKjJfA2gHkSqcT1Zgi4q/PZp2hDdN8=",
        version = "v0.26.1",
    )
    go_repository(
        name = "com_github_go_swagger_scan_repo_boundary",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-swagger/scan-repo-boundary",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:l9rI6sNaZgNC0LnF3MiE+qTmyBA/tZAg1rtyrGbUMK0=",
        version = "v0.0.0-20180623220736-973b3573c013",
    )
    go_repository(
        name = "com_github_go_test_deep",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-test/deep",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u2CU3YKy9I2pmu9pX0eq50wCgjfGIt539SqR7FbHiho=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_go_zookeeper_zk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/go-zookeeper/zk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4mx0EYENAdX/B/rbunjlt5+4RTA/a9SMHBRuSKdGxPM=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_gobuffalo_attrs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/attrs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hSkbZ9XSyjyBirMeqSqUrK+9HboWrweVlzRNqoBi2d4=",
        version = "v0.0.0-20190224210810-a9411de4debd",
    )
    go_repository(
        name = "com_github_gobuffalo_depgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/depgen",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:31atYa/UW9V5q8vMJ+W6wd64OaaTHUrCUXER358zLM4=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_gobuffalo_envy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/envy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GlXgaiBkmrYMHco6t4j7SacKO4XUjvh5pwXh0f4uxXU=",
        version = "v1.7.0",
    )
    go_repository(
        name = "com_github_gobuffalo_flect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/flect",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3GQ53z7E3o00C/yy7Ko8VXqQXoJGLkrTQCLTF1EjoXU=",
        version = "v0.1.3",
    )
    go_repository(
        name = "com_github_gobuffalo_genny",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/genny",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iQ0D6SpNXIxu52WESsD+KoQ7af2e3nCfnSBoSF/hKe0=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_gobuffalo_gitgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/gitgen",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mSVZ4vj4khv+oThUfS+SQU3UuFIZ5Zo6UNcvK8E8Mz8=",
        version = "v0.0.0-20190315122116-cc086187d211",
    )
    go_repository(
        name = "com_github_gobuffalo_gogen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/gogen",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dLg+zb+uOyd/mKeQUYIbwbNmfRsr9hd/WtYWepmayhI=",
        version = "v0.1.1",
    )
    go_repository(
        name = "com_github_gobuffalo_logger",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/logger",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8thhT+kUJMTMy3HlX4+y9Da+BNJck+p109tqqKp7WDs=",
        version = "v0.0.0-20190315122211-86e12af44bc2",
    )
    go_repository(
        name = "com_github_gobuffalo_mapi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/mapi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fq9WcL1BYrm36SzK6+aAnZ8hcp+SrmnDyAxhNx8dvJk=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_gobuffalo_packd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/packd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4sGKOD8yaYJ+dek1FDkwcxCHA40M4kfKgFHx8N2kwbU=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_gobuffalo_packr_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/packr/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Ir9W9XIm9j7bhhkKE9cokvtTl1vBm62A/fene/ZCj6A=",
        version = "v2.2.0",
    )
    go_repository(
        name = "com_github_gobuffalo_syncx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobuffalo/syncx",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tpom+2CJmpzAWj5/VEHync2rJGi+epHNIeRSWjzGA+4=",
        version = "v0.0.0-20190224160051-33c29581e754",
    )
    go_repository(
        name = "com_github_gobwas_httphead",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobwas/httphead",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:s+21KNqlpePfkah2I+gwHF8xmJWRjooY+5248k6m4A0=",
        version = "v0.0.0-20180130184737-2c6c146eadee",
    )
    go_repository(
        name = "com_github_gobwas_pool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobwas/pool",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QEmUOlnSjWtnpRGHF3SauEiOsy82Cup83Vf2LcMlnc8=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_gobwas_ws",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gobwas/ws",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CoAavW/wd/kulfZmSIBt6p24n4j7tHgNVCjsfHVNUbo=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_goccy_go_json",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/goccy/go-json",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/pAaQDLHEoCq/5FFmSKBswWmK6H0e8g4159Kc/X/nqk=",
        version = "v0.9.11",
    )
    go_repository(
        name = "com_github_godbus_dbus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/godbus/dbus",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BWhy2j3IXJhjCbC68FptL43tDKIq8FladmaTs3Xs7Z8=",
        version = "v0.0.0-20190422162347-ade71ed3457e",
    )
    go_repository(
        name = "com_github_godbus_dbus_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/godbus/dbus/v5",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mkgN1ofwASrYnJ5W6U/BxG15eXXXjirgZc7CLqkcaro=",
        version = "v5.0.6",
    )
    go_repository(
        name = "com_github_gofrs_flock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gofrs/flock",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+gYjHKf32LDeiEEFhQaotPbLuUXjY5ZqxKgXy7n59aw=",
        version = "v0.8.1",
    )
    go_repository(
        name = "com_github_gofrs_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gofrs/uuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1SD/1F5pU8p29ybwgQSwpQk+mwdRrXCYuPhW6m+TnJw=",
        version = "v4.0.0+incompatible",
    )
    go_repository(
        name = "com_github_gogo_googleapis",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogo/googleapis",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1Yx4Myt7BxzvUr5ldGSbwYiZG6t9wGBZ+8/fX3Wvtq0=",
        version = "v1.4.1",
    )
    go_repository(
        name = "com_github_gogo_protobuf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogo/protobuf",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_gogo_protobuf.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_gogo_status",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gogo/status",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+eIkrewn5q6b30y+g/BJINVVdi2xH7je5MPJ3ZPK3JA=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_golang_commonmark_html",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-commonmark/html",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FeNEDxIy7XouGTJKiJ9Ze5vUbcAIW/FRhQbtKBNmEz8=",
        version = "v0.0.0-20180910111043-7d7c804e1d46",
    )
    go_repository(
        name = "com_github_golang_commonmark_linkify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-commonmark/linkify",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TkuRzcq232K5ytXtQ+BPicsjYWZgt/lS6gJ5HqcUifQ=",
        version = "v0.0.0-20180910111149-f05efb453a0e",
    )
    go_repository(
        name = "com_github_golang_commonmark_markdown",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-commonmark/markdown",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YaQaotRjMcVth1VzHUEQlD2oeyQAglA7CXdxp9QLvKM=",
        version = "v0.0.0-20180910011815-a8f139058164",
    )
    go_repository(
        name = "com_github_golang_commonmark_mdurl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-commonmark/mdurl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XkgfhPs5AotQfcu3EfDEjyAUx91KdtjrxHXYGnZJhoU=",
        version = "v0.0.0-20180910110917-8d018c6567d6",
    )
    go_repository(
        name = "com_github_golang_commonmark_puny",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-commonmark/puny",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DUgQdQmDg4sk4SfNR+qOkXcopGz36BL02vp/V7WbPQI=",
        version = "v0.0.0-20180910110745-050be392d8b8",
    )
    go_repository(
        name = "com_github_golang_freetype",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/freetype",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DACJavvAHhabrF08vX0COfcOBJRhZ8lUbR+ZWIs0Y5g=",
        version = "v0.0.0-20170609003504-e2365dfdc4a0",
    )
    go_repository(
        name = "com_github_golang_geo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/geo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:C/hKUcHT483btRbeGkrRjJz+Zbcj8audldIi9tRJDCc=",
        version = "v0.0.0-20200319012246-673a6f80352d",
    )
    go_repository(
        name = "com_github_golang_glog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/glog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DrW6hGnjIhtvhOIiAKT6Psh/Kd/ldepEa81DKeiRJ5I=",
        version = "v1.2.5",
    )
    go_repository(
        name = "com_github_golang_groupcache",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/groupcache",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oI5xCqsCo564l8iNU+DwB5epxmsaqB+rhGL0m5jtYqE=",
        version = "v0.0.0-20210331224755-41bb18bfe9da",
    )
    go_repository(
        name = "com_github_golang_jwt_jwt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-jwt/jwt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IfV12K8xAKAnZqdXVzCZ+TOjboZ2keLg81eXfW3O+oY=",
        version = "v3.2.2+incompatible",
    )
    go_repository(
        name = "com_github_golang_jwt_jwt_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-jwt/jwt/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:besgBTC8w8HjP6NzQdxwKH9Z5oQMZ24ThTrHp3cZ8eU=",
        version = "v4.2.0",
    )
    go_repository(
        name = "com_github_golang_mock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/mock",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ErTB+efbowRARo13NNdxyJji2egdxLGQhRaY+DUumQc=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_golang_protobuf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/protobuf",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_golang_protobuf.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i7eJL8qZTpSEXOPTxNKhASYpMn+8e5Q6AdndVa1dWek=",
        version = "v1.5.4",
    )
    go_repository(
        name = "com_github_golang_snappy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang/snappy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yAGX7huGHXlcLOEtBnF4w7FQwA26wojNCwOYAEhLjQM=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_golang_sql_civil",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-sql/civil",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lXe2qZdvpiX5WZkZR4hgp4KJVfY3nMkvmwbVkpv1rVY=",
        version = "v0.0.0-20190719163853-cb61b32ac6fe",
    )
    go_repository(
        name = "com_github_golang_sql_sqlexp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golang-sql/sqlexp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+eHOFJl1BaXrQxKX+T06f78590z4qA2ZzBTqahsKSE4=",
        version = "v0.0.0-20170517235910-f1bb20e5a188",
    )
    go_repository(
        name = "com_github_golangci_lint_1",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/golangci/lint-1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:utua3L2IbQJmauC5IXdEA547bcoU5dozgQAfc8Onsg4=",
        version = "v0.0.0-20181222135242-d2cdd8c08219",
    )
    go_repository(
        name = "com_github_gomodule_redigo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gomodule/redigo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:y0Wmhvml7cGnzPa9nocn/fMraMH/lMDdeG+rkx4VgYY=",
        version = "v1.7.1-0.20190724094224-574c33c3df38",
    )
    go_repository(
        name = "com_github_gonum_blas",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gonum/blas",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Q0Jsdxl5jbxouNs1TQYt0gxesYMU4VXRbsTlgDloZ50=",
        version = "v0.0.0-20181208220705-f22b278b28ac",
    )
    go_repository(
        name = "com_github_gonum_floats",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gonum/floats",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EvokxLQsaaQjcWVWSV38221VAK7qc2zhaO17bKys/18=",
        version = "v0.0.0-20181209220543-c233463c7e82",
    )
    go_repository(
        name = "com_github_gonum_internal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gonum/internal",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8jtTdc+Nfj9AR+0soOeia9UZSvYBvETVHZrugUowJ7M=",
        version = "v0.0.0-20181124074243-f884aa714029",
    )
    go_repository(
        name = "com_github_gonum_lapack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gonum/lapack",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7qnwS9+oeSiOIsiUMajT+0R7HR6hw5NegnKPmn/94oI=",
        version = "v0.0.0-20181123203213-e4cdc5a0bff9",
    )
    go_repository(
        name = "com_github_gonum_matrix",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gonum/matrix",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:V2IgdyerlBa/MxaEFRbV5juy/C3MGdj4ePi+g6ePIp4=",
        version = "v0.0.0-20181209220409-c518dec07be9",
    )
    go_repository(
        name = "com_github_google_btree",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/btree",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gK4Kx5IaGY9CD5sPJ36FHiBJ6ZXl0kilRiiCj+jdYp4=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_google_flatbuffers",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/flatbuffers",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dicJ2oXwypfwUGnB2/TYWYEKiuk9eYQlQO/AnOHl5mI=",
        version = "v2.0.0+incompatible",
    )
    go_repository(
        name = "com_github_google_go_cmp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-cmp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wk8382ETsv4JYUZwIsn6YpYiWiBsYLSJiTsyBybVuN8=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_google_go_github",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:N0LgJ1j65A7kfXrZnUDaYCs/Sf4rEjNlfyDHW9dolSY=",
        version = "v17.0.0+incompatible",
    )
    go_repository(
        name = "com_github_google_go_github_v27",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github/v27",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:N/EEqsvJLgqTbepTiMBz+12KhwLovv6YvwpRezd+4Fg=",
        version = "v27.0.4",
    )
    go_repository(
        name = "com_github_google_go_github_v39",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github/v39",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pygGA5ySwxEez1N39GnDauD0PaWWuGgayudyZAc941s=",
        version = "v39.0.0",
    )
    go_repository(
        name = "com_github_google_go_github_v42",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-github/v42",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YNT0FwjPrEysRkLIiKuEfSvBPCGKphW5aS5PxwaoLec=",
        version = "v42.0.0",
    )
    go_repository(
        name = "com_github_google_go_pkcs11",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-pkcs11",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:PVRnTgtArZ3QQqTGtbtjtnIkzl2iY2kt24yqbrf7td8=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_google_go_querystring",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/go-querystring",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AnCroh3fv4ZBgVIf1Iwtovgjaw/GiKJo8M8yD/fhyJ8=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_google_gofuzz",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/gofuzz",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Hsa8mG0dQ46ij8Sl2AYJDUv1oA9/d6Vk+3LG99Oe02g=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_google_martian",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/martian",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/CP5g8u/VJHijgedC/Legn3BAbAaWPgecwXBIDzw5no=",
        version = "v2.1.0+incompatible",
    )
    go_repository(
        name = "com_github_google_martian_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/martian/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IqNFLAmvJOgVlpdEBiQbDc2EwKW77amAycfTuWKdfvw=",
        version = "v3.3.2",
    )
    go_repository(
        name = "com_github_google_pprof",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/pprof",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JCrt5MIE1fHQtdy1825HwJ45oVQaqHE6lgssRhjcg/o=",
        version = "v0.0.0-20210827144239-02619b876842",
    )
    go_repository(
        name = "com_github_google_renameio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/renameio",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GOZbcHa3HfsPKPlmyPyN2KEohoMXOhdMbHrvbpl2QaA=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_google_s2a_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/s2a-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:60BLSyTrOV4/haCDW4zb1guZItoSq8foHCXrAnjBo/o=",
        version = "v0.1.7",
    )
    go_repository(
        name = "com_github_google_safehtml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/safehtml",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ZOt2VXg4x24bW0m2jtzAOkhoXV0iM8vNKc0paByCZqM=",
        version = "v0.0.2",
    )
    go_repository(
        name = "com_github_google_shlex",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/shlex",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:El6M4kTTCOh6aBiKaUGG7oYTSPP8MxqL4YI3kZKwcP4=",
        version = "v0.0.0-20191202100458-e7afc7fbc510",
    )
    go_repository(
        name = "com_github_google_skylark",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/skylark",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rZdD1GeRTHD1aG+VIvhQEYXurx6Wfg4QIT5YVl2tSC8=",
        version = "v0.0.0-20181101142754-a5f7082aabed",
    )
    go_repository(
        name = "com_github_google_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/google/uuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_googleapis_enterprise_certificate_proxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/enterprise-certificate-proxy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yh8ncqsbUY4shRD5dA6RlzjJaT4hi3kII+zYw8wmLb8=",
        version = "v0.3.14",
    )
    go_repository(
        name = "com_github_googleapis_gax_go_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/gax-go/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9gWcmF85Wvq4ryPFvGFaOgPIs1AQX0d0bcbGw4Z96qg=",
        version = "v2.12.4",
    )
    go_repository(
        name = "com_github_googleapis_gnostic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/gnostic",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9fHAtK0uDfpveeqqo1hkEZJcFvYXAiCN3UutL8F9xHw=",
        version = "v0.5.5",
    )
    go_repository(
        name = "com_github_googleapis_google_cloud_go_testing",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/googleapis/google-cloud-go-testing",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tlyzajkF3030q6M8SvmJSemC9DTHL/xaMa18b65+JM4=",
        version = "v0.0.0-20200911160855-bcd43fbb19e8",
    )
    go_repository(
        name = "com_github_googlecloudplatform_cloudsql_proxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GoogleCloudPlatform/cloudsql-proxy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8F6fjL5iQP6sArGtPuXh0l6hggdcIpAm4ChjVJE4oTs=",
        version = "v0.0.0-20190129172621-c8b1d7a94ddf",
    )
    go_repository(
        name = "com_github_googlecloudplatform_opentelemetry_operations_go_detectors_gcp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rIkQfkCOVKc1OiRCNcSDD8ml5RJlZbH/Xsq7lbpynwc=",
        version = "v1.32.0",
    )
    go_repository(
        name = "com_github_gophercloud_gophercloud",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gophercloud/gophercloud",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1+4jrsjVhdX5omlAo4jkmFc6ftLbuXLzgFo4i6lH+Gk=",
        version = "v0.20.0",
    )
    go_repository(
        name = "com_github_gopherjs_gopherjs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gopherjs/gopherjs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JKmoR8x90Iww1ks85zJ1lfDGgIiMDuIptTOhJq+zKyg=",
        version = "v0.0.0-20181103185306-d547d1d9531e",
    )
    go_repository(
        name = "com_github_gordonklaus_ineffassign",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gordonklaus/ineffassign",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vc7Dmrk4JwS0ZPS6WZvWlwDflgDTA26jItmbSj83nug=",
        version = "v0.0.0-20200309095847-7953dde2c7bf",
    )
    go_repository(
        name = "com_github_gorilla_context",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/context",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AWwleXJkX/nhcU9bZSnZoi3h/qGYqQAGhq6zZe/aQW8=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_gorilla_css",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/css",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BQqNyPTi50JCFMTw/b67hByjMVXZRwGha6wxVGkeihY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_gorilla_handlers",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/handlers",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9lRY6j8DEeeBT10CvO9hGW0gmky0BprnvDI5vfhUHH4=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_gorilla_mux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/mux",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i40aqfkR1h2SlN9hojwV5ZA91wcXFOvkdNIeFDP5koI=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_gorilla_securecookie",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/securecookie",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:miw7JPhV+b/lAHSXz4qd/nN9jRiAFV5FwjeKyCS8BvQ=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_gorilla_sessions",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/sessions",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DHd3rPN5lE3Ts3D8rKkQ8x/0kqfeNmBAaiSi+o7FsgI=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_gorilla_websocket",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gorilla/websocket",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+/TMaTYc4QFitKJxsQ7Yye35DkWvkdLcvGKqM+x0Ufc=",
        version = "v1.4.2",
    )
    go_repository(
        name = "com_github_goware_modvendor",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/goware/modvendor",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3XXkmWdTccMzBswM5FTTXvWEtCV7DP7VRkIACRCGaqU=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_gregjones_httpcache",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/gregjones/httpcache",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pdN6V1QBWetyv/0+wjACpqVH+eVULgEjkurDLq3goeM=",
        version = "v0.0.0-20180305231024-9cad4c3443a7",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_go_grpc_middleware",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/go-grpc-middleware",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:THDBEeQ9xZ8JEaCLyLQqXMMdRqNr0QAUJTIkQAUtFjg=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_go_grpc_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/grpc-ecosystem/go-grpc-prometheus",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Ovs26xHkKqVztRpIrF/92BcuyuQ/YW4NSIpoGtfXNho=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_grpc_ecosystem_grpc_gateway",
        build_file_proto_mode = "disable_global",
        build_naming_convention = "go_default_library",
        importpath = "github.com/grpc-ecosystem/grpc-gateway",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_grpc_ecosystem_grpc_gateway.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gmcG1KaJ57LophUzW0Hy8NmPhnMZb4M0+kPpLofRdBo=",
        version = "v1.16.0",
    )
    go_repository(
        name = "com_github_guptarohit_asciigraph",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/guptarohit/asciigraph",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ccFnUF8xYIOUPPY3tmdvRyHqmn1MYI9iv1pLKX+/ZkQ=",
        version = "v0.5.5",
    )
    go_repository(
        name = "com_github_hashicorp_consul_api",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/consul/api",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MwZJp86nlnL+6+W1Zly4JUuVn9YHhMggBirMpHGD7kw=",
        version = "v1.10.1",
    )
    go_repository(
        name = "com_github_hashicorp_consul_sdk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/consul/sdk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OJtKBtEjboEZvG6AOUdh4Z1Zbyu0WcxQ0qatRrZHTVU=",
        version = "v0.8.0",
    )
    go_repository(
        name = "com_github_hashicorp_errwrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/errwrap",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OxrOeh75EUXMY8TBjag2fzXGZ40LB6IKw45YeGUDY2I=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_hashicorp_go_cleanhttp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-cleanhttp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dH3aiDG9Jvb5r5+bYHsikaOUIpcM0xvgMXVoDkXMzJM=",
        version = "v0.5.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_hclog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-hclog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:F1fdYblUEsxKiailtkhCCG2g4bipEgaHiDc8vffNpD4=",
        version = "v0.12.2",
    )
    go_repository(
        name = "com_github_hashicorp_go_immutable_radix",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-immutable-radix",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:l6UW37iCXwZkZoAbEYnptSHVE/cQ5bOTPYG5W3vf9+8=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_hashicorp_go_msgpack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-msgpack",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i9R9JSrqIz0QVLz3sz+i3YJdT7TTSLcfLLzJi9aZTuI=",
        version = "v0.5.5",
    )
    go_repository(
        name = "com_github_hashicorp_go_multierror",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-multierror",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:H5DkEtf6CXdFp0N0Em5UCwQpXMWke8IA0+lD48awMYo=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_net",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go.net",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sNCoNyDEvN1xa+X0baata4RdcpKwcMS6DH+xwfqPgjw=",
        version = "v0.0.1",
    )
    go_repository(
        name = "com_github_hashicorp_go_retryablehttp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-retryablehttp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QlWt0KvWT0lq8MFppF9tsJGF+ynG7ztc2KIPhzRGk7s=",
        version = "v0.5.3",
    )
    go_repository(
        name = "com_github_hashicorp_go_rootcerts",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-rootcerts",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jzhAVGtqPKbwpyCPELlgNWhE1znq+qwJtW5Oi2viEzc=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_hashicorp_go_sockaddr",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-sockaddr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ztczhD1jLxIRjVejw8gFomI1BQZOe2WoVOu0SyteCQc=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_hashicorp_go_syslog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-syslog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KaodqZuhUoZereWVIYmpUgZysurB1kBLX2j0MwMrUAE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_hashicorp_go_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-uuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2gKiV6YVmrJ1i2CKKa9obLvRieoRGviZFL26PcT/Co8=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_hashicorp_go_version",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/go-version",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3vNe/fWF5CBgRIguda1meWhsZHy3m8gCJ5wx+dIzX/E=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_hashicorp_golang_lru",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/golang-lru",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YDjusn29QI/Das2iO9M0BHnIbxPeyuCHsjMW+lJfyTc=",
        version = "v0.5.4",
    )
    go_repository(
        name = "com_github_hashicorp_hcl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/hcl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0Anlzjpi4vEasTeNFn2mLJgTSwt0+6sfsiTG8qcWGx4=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_hashicorp_logutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/logutils",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dLEQVugN8vlakKOUE3ihGLTZJRB4j+M2cdTm/ORI65Y=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_hashicorp_mdns",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/mdns",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XFSOubp8KWB+Jd2PDyaX5xUd5bhSP/+pTDZVDMzZJM8=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_hashicorp_memberlist",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/memberlist",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OOhYzSvFnkFQXm1ysE8RjXTHsqSRDyP4emusC9K7DYg=",
        version = "v0.2.4",
    )
    go_repository(
        name = "com_github_hashicorp_serf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hashicorp/serf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EBWvyu9tcRszt3Bxp3KNssBMP1KuHWyO51lz9+786iM=",
        version = "v0.9.5",
    )
    go_repository(
        name = "com_github_hdrhistogram_hdrhistogram_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/HdrHistogram/hdrhistogram-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5IcZpTvzydCQeHzK4Ef/D5rrSqwxob0t8PQPMybUNFM=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_hetznercloud_hcloud_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hetznercloud/hcloud-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7zyN2V7hMlhm3HZdxOarmOtvzKvkcYKjM0hcwYMQZz0=",
        version = "v1.32.0",
    )
    go_repository(
        name = "com_github_howeyc_gopass",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/howeyc/gopass",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:aY2hhxLhjEAbfXOx2nRJxCXezC6CO2V/yN+OCr1srtk=",
        version = "v0.0.0-20190910152052-7cb4b85ec19c",
    )
    go_repository(
        name = "com_github_hpcloud_tail",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hpcloud/tail",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nfCOvKYfkgYP8hkirhJocXT2+zOD8yUNjXaWfTlyFKI=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_huandu_xstrings",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/huandu/xstrings",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gvV6jG9dTgFEncxo+AF7PH6MZXi/vZl25owA/8Dg8Wo=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_hudl_fargo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hudl/fargo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0U6+BtN6LhaYuTnIJq4Wyq5cpn6O2kWrxAtcqBmYY6w=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_hydrogen18_memlistener",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/hydrogen18/memlistener",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JR7eDj8HD6eXrc5fWLbSUnfcQFL06PYvCc0DKQnWfaU=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_iancoleman_strcase",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iancoleman/strcase",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nTXanmYxhfFAMjZL34Ov6gkzEsSJZ5DbhxWjvSASxEI=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_ianlancetaylor_demangle",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ianlancetaylor/demangle",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mV02weKRL81bEnm8A0HT1/CAelMQDBuQIfLw8n+d6xI=",
        version = "v0.0.0-20200824232613-28f6c0f3b639",
    )
    go_repository(
        name = "com_github_icrowley_fake",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/icrowley/fake",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Mo9W14pwbO9VfRe+ygqZ8dFbPpoIK1HFrG/zjTuQ+nc=",
        version = "v0.0.0-20180203215853-4178557ae428",
    )
    go_repository(
        name = "com_github_imdario_mergo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/imdario/mergo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lFzP57bqS/wsqKssCGmtLAb8A0wKjLGrve2q3PPVcBk=",
        version = "v0.3.13",
    )
    go_repository(
        name = "com_github_imkira_go_interpol",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/imkira/go-interpol",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KIiKr0VSG2CUW1hl1jpiyuzuJeKUUpC8iM1AIE7N1Vk=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_inconshreveable_mousetrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/inconshreveable/mousetrap",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Z8tu5sraLXCXIcARxBp/8cbvlwVa7Z1NHg9XEKhtSvM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_influxdata_flux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/flux",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:M4x6e25+ao95N98kB65wd59juA+RV7WDhcsYuxL5/6M=",
        version = "v0.120.1",
    )
    go_repository(
        name = "com_github_influxdata_httprouter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/httprouter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WQsmW0fXO4ZE/lFGIE84G6rIV5SJN3P3sjIXAP1a8eU=",
        version = "v1.3.1-0.20191122104820-ee83e2772f69",
    )
    go_repository(
        name = "com_github_influxdata_influxdb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/influxdb",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:60F7eqotCxogyuZAjNglNRG9D6WY65KR9mcmugBx6cs=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_github_influxdata_influxdb1_client",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/influxdb1-client",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/WZQPMZNsjZ7IlCpsLGdQBINg5bxKQ1K1sh6awxLtkA=",
        version = "v0.0.0-20191209144304-8bf82d3c094d",
    )
    go_repository(
        name = "com_github_influxdata_influxdb_client_go_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/influxdb-client-go/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MBLCfcSsUyFPDJp6T7EoHp/Ph3Jkrm4EuUKLD2rUWHg=",
        version = "v2.3.1-0.20210518120617-5d1fff431040",
    )
    go_repository(
        name = "com_github_influxdata_influxql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/influxql",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4t/8PcmLnI2vrcaHcEKeeLsGxC0WMRaOQdPX9b7DF8Y=",
        version = "v1.1.1-0.20210223160523-b6ab99450c93",
    )
    go_repository(
        name = "com_github_influxdata_line_protocol",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/line-protocol",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:W9WBk7wlPfJLvMCdtV4zPulc4uCPrlywQOmbFOhgQNU=",
        version = "v0.0.0-20200327222509-2487e7298839",
    )
    go_repository(
        name = "com_github_influxdata_pkg_config",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/pkg-config",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LPTCWmcPkyMryHHnf+STK/zVUjQ6OCvvOukSDlJLY9I=",
        version = "v0.2.7",
    )
    go_repository(
        name = "com_github_influxdata_promql_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/promql/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kXn3p0D7zPw16rOtfDR+wo6aaiH8tSMfhPwONTxrlEc=",
        version = "v2.12.0",
    )
    go_repository(
        name = "com_github_influxdata_roaring",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/roaring",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UzJnB7VRL4PSkUJHwsyzseGOmrO/r4yA+AuxGJxiZmA=",
        version = "v0.4.13-0.20180809181101-fc520f41fab6",
    )
    go_repository(
        name = "com_github_influxdata_tdigest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/tdigest",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i44CesU68ZBRvtCjBi3QSosCIKrjmMbYlQMFAwVLds4=",
        version = "v0.0.2-0.20210216194612-fc98d27c9e8b",
    )
    go_repository(
        name = "com_github_influxdata_usage_client",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/influxdata/usage-client",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+TUUmaFa4YD1Q+7bH9o5NCHQGPMqZCYJiNW6lIIS9z4=",
        version = "v0.0.0-20160829180054-6d3895376368",
    )
    go_repository(
        name = "com_github_irfansharif_recorder",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/irfansharif/recorder",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fwL5Wt/OpP14udrdhV+INmmRES4GWoPWqHamttadwKU=",
        version = "v0.0.0-20211218081646-a21b46510fd6",
    )
    go_repository(
        name = "com_github_iris_contrib_blackfriday",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iris-contrib/blackfriday",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:o5sHQHHm0ToHUlAJSTjW9UWicjJSDDauOOQ2AHuIVp4=",
        version = "v2.0.0+incompatible",
    )
    go_repository(
        name = "com_github_iris_contrib_go_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iris-contrib/go.uuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XZubAYg61/JwnJNbZilGjf3b3pB80+OQg2qf6c8BfWE=",
        version = "v2.0.0+incompatible",
    )
    go_repository(
        name = "com_github_iris_contrib_jade",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iris-contrib/jade",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:p7J/50I0cjo0wq/VWVCDFd8taPJbuFC+bq23SniRFX0=",
        version = "v1.1.3",
    )
    go_repository(
        name = "com_github_iris_contrib_pongo2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iris-contrib/pongo2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zGP7pW51oi5eQZMIlGA3I+FHY9/HOQWDB+572yin0to=",
        version = "v0.0.1",
    )
    go_repository(
        name = "com_github_iris_contrib_schema",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/iris-contrib/schema",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CPSBLyx2e91H2yJzPuhGuifVRnZBBJ3pCOMbOvPZaTw=",
        version = "v0.0.6",
    )
    go_repository(
        name = "com_github_j_keck_arping",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/j-keck/arping",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:742eGXur0715JMq73aD95/FU0XpVKXqNuTnEfXsLOYQ=",
        version = "v0.0.0-20160618110441-2cf9dc699c56",
    )
    go_repository(
        name = "com_github_jackc_chunkreader",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/chunkreader",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4s39bBR8ByfqH+DKm8rQA3E1LHZWB9XWcrz8fqaZbe0=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jackc_chunkreader_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/chunkreader/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i+RDz65UE+mmpjTfyz0MoVTnzeYxroil2G82ki7MGG8=",
        version = "v2.0.1",
    )
    go_repository(
        name = "com_github_jackc_pgconn",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgconn",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bPKk32KlGLQ/SWtOc3onTzZ/km0BIiwwYBqt+8YlKwU=",
        version = "v1.13.1-0.20221001150415-49cbf4659151",
    )
    go_repository(
        name = "com_github_jackc_pgio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgio",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:g12B9UwVnzGhueNavwioyEEpAmqMe1E/BN9ES+8ovkE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jackc_pgmock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgmock",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DadwsjnMwFjfWc9y5Wi/+Zz7xoE5ALHsRQlOctkOiHc=",
        version = "v0.0.0-20210724152146-4ad1a8207f65",
    )
    go_repository(
        name = "com_github_jackc_pgpassfile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgpassfile",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/6Hmqy13Ss2zCq62VdNG8tM1wchn8zjSGOBJ6icpsIM=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jackc_pgproto3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgproto3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FYYE4yRw+AgI8wXIinMlNjBbp/UitDJwfj5LqqewP1A=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_jackc_pgproto3_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgproto3/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nwj7qwf0S+Q7ISFfBndqeLwSwxs+4DPsbRFjECT1Y4Y=",
        version = "v2.3.1",
    )
    go_repository(
        name = "com_github_jackc_pgservicefile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgservicefile",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bbPeKD0xmW/Y25WS6cokEszi5g+S0QxI/d45PkRi7Nk=",
        version = "v0.0.0-20221227161230-091c0ba34f0a",
    )
    go_repository(
        name = "com_github_jackc_pgtype",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgtype",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u4uiGPz/1hryuXzyaBhSk6dnIyyG2683olG2OV+UUgs=",
        version = "v1.11.0",
    )
    go_repository(
        name = "com_github_jackc_pgx_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgx/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JzTglcal01DrghUqt+PmzWsZx/Yh7SC/CTQmSBMTd0Y=",
        version = "v4.16.1",
    )
    go_repository(
        name = "com_github_jackc_pgx_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/pgx/v5",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Fcr8QJ1ZeLi5zsPZqQeUZhNhxfkkKBOgJuYkJHoBOtU=",
        version = "v5.3.1",
    )
    go_repository(
        name = "com_github_jackc_puddle",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/puddle",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gI8os0wpRXFd4FiAY2dWiqRK037tjj3t7rKFeO4X5iw=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_jackc_puddle_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jackc/puddle/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RdcDk92EJBuBS55nQMMYFXTxwstHug4jkhT5pq8VxPk=",
        version = "v2.2.0",
    )
    go_repository(
        name = "com_github_jcmturner_aescts_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jcmturner/aescts/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9YKLH6ey7H4eDBXW8khjYslgyqG2xZikXP0EQFKrle8=",
        version = "v2.0.0",
    )
    go_repository(
        name = "com_github_jcmturner_dnsutils_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jcmturner/dnsutils/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lltnkeZGL0wILNvrNiVCR6Ro5PGU/SeBvVO/8c/iPbo=",
        version = "v2.0.0",
    )
    go_repository(
        name = "com_github_jcmturner_gofork",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jcmturner/gofork",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QH0l3hzAU1tfT3rZCnW5zXl+orbkNMMRGJfdJjHVETg=",
        version = "v1.7.6",
    )
    go_repository(
        name = "com_github_jcmturner_goidentity_v6",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jcmturner/goidentity/v6",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VKnZd2oEIMorCTsFBnJWbExfNN7yZr3EhJAxwOkZg6o=",
        version = "v6.0.1",
    )
    go_repository(
        name = "com_github_jcmturner_gokrb5_v8",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jcmturner/gokrb5/v8",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iTonLeSJOn7MVUtyMT+arAn5AKAPrkilzhGw8wE/Tq8=",
        version = "v8.4.3",
    )
    go_repository(
        name = "com_github_jcmturner_rpc_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jcmturner/rpc/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7FXXj8Ti1IaVFpSAziCZWNzbNuZmnvw/i6CqLNdWfZY=",
        version = "v2.0.3",
    )
    go_repository(
        name = "com_github_jessevdk_go_flags",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jessevdk/go-flags",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1jKYvbxEjfUl0fmqTCOfonvskHHXMjBySTLW4y9LFvc=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_jhump_protoreflect",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jhump/protoreflect",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Yb4fEGk+GtBSNuvy5rs0ZJt/jtopc/z9azQaj3xbies=",
        version = "v1.9.1-0.20210817181203-db1a327a393e",
    )
    go_repository(
        name = "com_github_jinzhu_inflection",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jinzhu/inflection",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:K317FqzuhWc8YvSVlFMCCUb36O/S9MCKRDI7QkRKD/E=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jinzhu_now",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jinzhu/now",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tHnRBy1i5F2Dh8BAFxqFzxKqqvezXrL2OW1TnX+Mlas=",
        version = "v1.1.4",
    )
    go_repository(
        name = "com_github_jmespath_go_jmespath",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jmespath/go-jmespath",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BEgLn5cpjn8UN1mAw4NjwDrS35OdebyEtFe+9YPoQUg=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_jmespath_go_jmespath_internal_testify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jmespath/go-jmespath/internal/testify",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:shLQSRRSCCPj3f2gpwzGwWFoC7ycTf1rcQZHOlsJ6N8=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_jmoiron_sqlx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jmoiron/sqlx",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vFFPA71p1o5gAeqtEAwLU4dnX2napprKtHr7PYIcN3g=",
        version = "v1.3.5",
    )
    go_repository(
        name = "com_github_joho_godotenv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/joho/godotenv",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Zjp+RcGpHhGlrMbJzXTrZZPrWj+1vfm90La1wgB6Bhc=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_joker_hpp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Joker/hpp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:65+iuJYdRXv/XyN62C1uEmmOx3432rNG/rKlX6V7Kkc=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_joker_jade",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Joker/jade",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Qbeh12Vq6BxURXT1qZBRHsDxeURB8ztcL6f3EXSGeHk=",
        version = "v1.1.3",
    )
    go_repository(
        name = "com_github_jonboulle_clockwork",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jonboulle/clockwork",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VKV+ZcuP6l3yW9doeqz6ziZGgcynBVQO+obU0+0hcPo=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_jordan_wright_email",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jordan-wright/email",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jdpOPRN1zP63Td1hDQbZW73xKmzDvZHzVdNYxhnTMDA=",
        version = "v4.0.1-0.20210109023952-943e75fe5223+incompatible",
    )
    go_repository(
        name = "com_github_jordanlewis_gcassert",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jordanlewis/gcassert",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GoV80Ggs5ZeF6uDv/lYDGJAv2Wz3Mp/brqRbjvEm6mI=",
        version = "v0.0.0-20230505190637-fed79d91cd5f",
    )
    go_repository(
        name = "com_github_josharian_intern",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/josharian/intern",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vlS4z54oSdjm0bgjRigI+G1HpF+tI+9rE5LLzOg8HmY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_jpillora_backoff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jpillora/backoff",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uvFg412JmmHBHw7iwprIxkPMI+sGQ4kzOWsMeHnm2EA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_json_iterator_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/json-iterator/go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:PV8peI4a0ysnczrg+LtxykD8LfKY9ML6u2jnxaEnrnM=",
        version = "v1.1.12",
    )
    go_repository(
        name = "com_github_jstemmer_go_junit_report",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jstemmer/go-junit-report",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6QPYqodiu3GuPL+7mfx+NwDdp2eTkp9IfEUpgAwUN0o=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_jsternberg_zap_logfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jsternberg/zap-logfmt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1v+PK4/B48cy8cfQbxL4FmmNZrjnIMr2BsnyEmXqv2o=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_jtolds_gls",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jtolds/gls",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xdiiI2gbIgH/gLH7ADydsJ1uDOEzR8yvV7C0MuV77Wo=",
        version = "v4.20.0+incompatible",
    )
    go_repository(
        name = "com_github_julienschmidt_httprouter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/julienschmidt/httprouter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:U0609e9tgbseu3rBINet9P48AI/D3oJs4dN7jwJOQ1U=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_julusian_godocdown",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Julusian/godocdown",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:n3F+mWm+b4D7uNbx1syN/uQTVDwt2sWfk23Mhzwzec4=",
        version = "v0.0.0-20170816220326-6d19f8ff2df8",
    )
    go_repository(
        name = "com_github_jung_kurt_gofpdf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jung-kurt/gofpdf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:PJr+ZMXIecYc1Ey2zucXdR73SMBtgjPgwa31099IMv0=",
        version = "v1.0.3-0.20190309125859-24315acbbda5",
    )
    go_repository(
        name = "com_github_justinas_alice",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/justinas/alice",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+MHSA/vccVCF4Uq37S42jwlkvI2Xzl7zTPCN5BnZNVo=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_jwilder_encoding",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/jwilder/encoding",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2jNeR4YUziVtswNP9sEFAI913cVrzH85T+8Q6LpYbT0=",
        version = "v0.0.0-20170811194829-b4e1701a28ef",
    )
    go_repository(
        name = "com_github_k0kubun_colorstring",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/k0kubun/colorstring",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uC1QfSlInpQF+M0ao65imhwqKnz3Q2z/d8PWZRMQvDM=",
        version = "v0.0.0-20150214042306-9440f1994b88",
    )
    go_repository(
        name = "com_github_karrick_godirwalk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/karrick/godirwalk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lOpSw2vJP0y5eLBW906QwKsUK/fe/QDyoqM5rnnuPDY=",
        version = "v1.10.3",
    )
    go_repository(
        name = "com_github_kataras_blocks",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/blocks",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cF3RDY/vxnSRezc7vLFlQFTYXG/yAr1o7WImJuZbzC4=",
        version = "v0.0.7",
    )
    go_repository(
        name = "com_github_kataras_golog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/golog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:isP8th4PJH2SrbkciKnylaND9xoTtfxv++NB+DF0l9g=",
        version = "v0.1.8",
    )
    go_repository(
        name = "com_github_kataras_iris_v12",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/iris/v12",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WzDY5nGuW/LgVaFS5BtTkW3crdSKJ/FEgWnxPnIVVLI=",
        version = "v12.2.0",
    )
    go_repository(
        name = "com_github_kataras_neffos",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/neffos",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pdJaTvUG3NQfeMbbVCI8JT2T5goPldyyfUB2PJfh1Bs=",
        version = "v0.0.14",
    )
    go_repository(
        name = "com_github_kataras_pio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/pio",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kqreJ5KOEXGMwHAWHDwIl+mjfNCPhAwZPa8gK7MKlyw=",
        version = "v0.0.11",
    )
    go_repository(
        name = "com_github_kataras_sitemap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/sitemap",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:w71CRMMKYMJh6LR2wTgnk5hSgjVNB9KL60n5e2KHvLY=",
        version = "v0.0.6",
    )
    go_repository(
        name = "com_github_kataras_tunnel",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kataras/tunnel",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sCAqWuJV7nPzGrlb0os3j49lk2JhILT0rID38NHNLpA=",
        version = "v0.0.4",
    )
    go_repository(
        name = "com_github_kballard_go_shellquote",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kballard/go-shellquote",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Z9n2FFNUXsshfwJMBgNA0RU6/i7WVaAegv3PtuIHPMs=",
        version = "v0.0.0-20180428030007-95032a82bc51",
    )
    go_repository(
        name = "com_github_kevinburke_go_bindata",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kevinburke/go-bindata",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hThDhUBH4KjTyhfXfOgacEPfFBNjltnzl/xzfLfrPoQ=",
        version = "v3.13.0+incompatible",
    )
    go_repository(
        name = "com_github_kisielk_errcheck",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kisielk/errcheck",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_kisielk_errcheck.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:o2feyvFn8r1nJu/iVn89h0kT+CH6pROXZZHQO60NbrI=",
        version = "v1.6.1-0.20210625163953-8ddee489636a",
    )
    go_repository(
        name = "com_github_kisielk_gotool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kisielk/gotool",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AV2c/EiW3KqPNT9ZKl07ehoAGi4C5/01Cfbblndcapg=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_klauspost_compress",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/compress",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iULayQNOReoYUe+1qtKOqw9CwJv3aNQu8ivo7lw1HU4=",
        version = "v1.16.0",
    )
    go_repository(
        name = "com_github_klauspost_cpuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/cpuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5JNjFYYQrZeKRJ0734q51WCEEn2huer72Dc7K+R/b6s=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_klauspost_cpuid_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/cpuid/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lgaqFMSdTdQYdZ04uHyN2d/eKdOMyi2YLSvlQIBFYa4=",
        version = "v2.0.9",
    )
    go_repository(
        name = "com_github_klauspost_crc32",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/crc32",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KAZ1BW2TCmT6PRihDPpocIy1QTtsAsrx6TneU/4+CMg=",
        version = "v0.0.0-20161016154125-cb6bfca970f6",
    )
    go_repository(
        name = "com_github_klauspost_pgzip",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/klauspost/pgzip",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qnWYvvKqedOF2ulHpMG72XQol4ILEJ8k2wwRl/Km8oE=",
        version = "v1.2.5",
    )
    go_repository(
        name = "com_github_knetic_govaluate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Knetic/govaluate",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1G1pk05UrOh0NlF1oeaaix1x8XzrfjIDK47TY0Zehcw=",
        version = "v3.0.1-0.20171022003610-9aa49832a739+incompatible",
    )
    go_repository(
        name = "com_github_knz_bubbline",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knz/bubbline",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uhwfuoL9LDMUGlxcs3Rbt35L3YoqGUd+YWgaL9EFl0A=",
        version = "v0.0.0-20230422210153-e176cdfe1c43",
    )
    go_repository(
        name = "com_github_knz_catwalk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knz/catwalk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GgCxHbPp+nzyZBJcNL/CJd1aba4ACoeuI1lnsshAPkY=",
        version = "v0.1.4",
    )
    go_repository(
        name = "com_github_knz_go_libedit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knz/go-libedit",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:62iTw+D4JtoXL7yp4S5Ruyc82qmbWdnwuuSZ3tmcuSw=",
        version = "v1.10.2-0.20230621133438-5f2b2e7387c5",
    )
    go_repository(
        name = "com_github_knz_lipgloss_convert",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knz/lipgloss-convert",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qUPUt6r8mqvi9DIV3nBPu3kEmFyHrZtXzv0BlPBPLNQ=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_knz_strtime",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/knz/strtime",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_knz_strtime.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GQE1iatYDRrIidq4Zf/9ZzKWyrTk2sXOYc1JADbkAjQ=",
        version = "v0.0.0-20200318182718-be999391ffa9",
    )
    go_repository(
        name = "com_github_konsorten_go_windows_terminal_sequences",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/konsorten/go-windows-terminal-sequences",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CE8S1cTafDpPvMhIxNJKvHsGVBgn1xWYf1NbHQhywc8=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_kr_fs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/fs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Jskdu9ieNAYnjxsi0LbQp1ulIKZV1LAFgK1tWhpZgl8=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_kr_logfmt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/logfmt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:T+h1c/A9Gawja4Y9mFVWj2vyii2bbUNDw3kt9VxK2EY=",
        version = "v0.0.0-20140226030751-b84e30acd515",
    )
    go_repository(
        name = "com_github_kr_pretty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/pretty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:flRD4NNwYAUpkphVc1HcthR4KEIFJ65n8Mw5qdRn3LE=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_kr_pty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/pty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AkaSdXYQOWeaO3neb8EM634ahkXXe3jYbVh/F9lq+GI=",
        version = "v1.1.8",
    )
    go_repository(
        name = "com_github_kr_text",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kr/text",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5Nx0Ya0ZqY2ygV366QzturHI13Jq95ApcVaJBhpS+AY=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_krishicks_yaml_patch",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/krishicks/yaml-patch",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:H4FcHpnNwVmw8u0MjPRjWyIXtco6zM2F78t+57oNM3E=",
        version = "v0.0.10",
    )
    go_repository(
        name = "com_github_kylelemons_godebug",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/kylelemons/godebug",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RPNrshWIDI6G2gRW9EHilWtl7Z6Sb1BR0xunSBf0SNc=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_labstack_echo_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/labstack/echo/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5CiyngihEO4HXsz3vVsJn7f8xAlWwRr3aY6Ih280ZKA=",
        version = "v4.10.0",
    )
    go_repository(
        name = "com_github_labstack_gommon",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/labstack/gommon",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:y7cvthEAEbU0yHOf4axH8ZG2NH8knB9iNSoTO8dyIk8=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_leanovate_gopter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/leanovate/gopter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+OUpk+IVvmKU0jivOVFGtOzA6U5AWFs8HE4DRzWLOUE=",
        version = "v0.2.5-0.20190402064358-634a59d12406",
    )
    go_repository(
        name = "com_github_leodido_go_urn",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/leodido/go-urn",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BqpAaACuzVSgi/VLzGZIobT2z4v53pjosyNd9Yv6n/w=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_lestrrat_go_backoff_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lestrrat-go/backoff/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oNb5E5isby2kiro9AgdHLv5N5tint1AnDVVf2E2un5A=",
        version = "v2.0.8",
    )
    go_repository(
        name = "com_github_lestrrat_go_blackmagic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lestrrat-go/blackmagic",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lS5Zts+5HIC/8og6cGHb0uCcNCa3OUt1ygh3Qz2Fe80=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_lestrrat_go_httpcc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lestrrat-go/httpcc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ydWCStUeJLkpYyjLDHihupbn2tYmZ7m22BGkcvZZrIE=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_lestrrat_go_iter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lestrrat-go/iter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gMXo1q4c2pHmC3dn8LzRhJfP1ceCbgSiT9lUydIzltI=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_lestrrat_go_jwx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lestrrat-go/jwx",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tAx93jN2SdPvFn08fHNAhqFJazn5mBBOB8Zli0g0otA=",
        version = "v1.2.25",
    )
    go_repository(
        name = "com_github_lestrrat_go_option",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lestrrat-go/option",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WqAWL8kh8VcSoD6xjSH34/1m8yxluXQbDeKNfvFeEO4=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_lib_pq",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lib/pq",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:p7ZhMD+KsSRozJr34udlUrhboJwWAgCg34+/ZZNvZZw=",
        version = "v1.10.7",
    )
    go_repository(
        name = "com_github_lightstep_lightstep_tracer_common_golang_gogo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lightstep/lightstep-tracer-common/golang/gogo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:143Bb8f8DuGWck/xpNUOckBVYfFbBTnLevfRZ1aVVqo=",
        version = "v0.0.0-20190605223551-bc2310a04743",
    )
    go_repository(
        name = "com_github_lightstep_lightstep_tracer_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lightstep/lightstep-tracer-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vi1F1IQ8N7hNWytK9DpJsUfQhGuNSc19z330K6vl4zk=",
        version = "v0.18.1",
    )
    go_repository(
        name = "com_github_linkedin_goavro_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/linkedin/goavro/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rIQQSj8jdAUlKQh6DttK8wCRv4t4QO09g1C4aBWXslg=",
        version = "v2.12.0",
    )
    go_repository(
        name = "com_github_linode_linodego",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/linode/linodego",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IK04cx2b/IwAAd6XLruf1Dl/n3dRXj87Uw/5qo6afVU=",
        version = "v0.32.0",
    )
    go_repository(
        name = "com_github_lucasb_eyer_go_colorful",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lucasb-eyer/go-colorful",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1nnpGOrhyZZuNyfu1QjKiUICQ74+3FNCN69Aj6K7nkY=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_lufia_iostat",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lufia/iostat",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tnCdZBIglgxD47RyD55kfWQcJMGzO+1QBziSQfesf2k=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_lufia_plan9stats",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lufia/plan9stats",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6E+4a0GO5zZEnZ81pIr0yLvtUWk2if982qA3F3QD6H4=",
        version = "v0.0.0-20211012122336-39d0f177ccd0",
    )
    go_repository(
        name = "com_github_lyft_protoc_gen_star",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lyft/protoc-gen-star",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sImehRT+p7lW9n6R7MQc5hVgzWGEkDVZU4AsBQ4Isu8=",
        version = "v0.5.1",
    )
    go_repository(
        name = "com_github_lyft_protoc_gen_star_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lyft/protoc-gen-star/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sIXJOMrYnQZJu7OB7ANSF4MYri2fTEGIsRLz6LwI4xE=",
        version = "v2.0.4-0.20230330145011-496ad1ac90a4",
    )
    go_repository(
        name = "com_github_lyft_protoc_gen_validate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/lyft/protoc-gen-validate",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KNt/RhmQTOLr7Aj8PsJ7mTronaFyx80mRTT9qF261dA=",
        version = "v0.0.13",
    )
    go_repository(
        name = "com_github_magiconair_properties",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/magiconair/properties",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:b6kJs+EmPFMYGkow9GiUyCyOvIwYetYJ3fSaWak/Gls=",
        version = "v1.8.5",
    )
    go_repository(
        name = "com_github_mailgun_raymond_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mailgun/raymond/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5dmlB680ZkFG2RN/0lvTAghrSxIESeu9/2aeDqACtjw=",
        version = "v2.0.48",
    )
    go_repository(
        name = "com_github_mailru_easyjson",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mailru/easyjson",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UGYAvKxe3sBsEDzO8ZeWOSlIQfWFlxbzLZe7hwFURr0=",
        version = "v0.7.7",
    )
    go_repository(
        name = "com_github_markbates_oncer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/markbates/oncer",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JgVTCPf0uBVcUSWpyXmGpgOc62nK5HWUBKAGc3Qqa5k=",
        version = "v0.0.0-20181203154359-bf2de49a0be2",
    )
    go_repository(
        name = "com_github_markbates_safe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/markbates/safe",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yjZkbvRM6IzKj9tlu/zMJLS0n/V351OZWRnF3QfaUxI=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_marstr_guid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/marstr/guid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/M4H/1G4avsieL6BbUwCOBzulmoeKVP5ux/3mQNnbyI=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_martini_contrib_auth",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/martini-contrib/auth",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1ded5x5QpCLsyTH5ct62Rh1RXPFnn0/dubCqAeh+stU=",
        version = "v0.0.0-20150219114609-fa62c19b7ae8",
    )
    go_repository(
        name = "com_github_martini_contrib_gzip",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/martini-contrib/gzip",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wVDxEVZP1eiPIlHVaafUAEUDtyl6ytjHv3egJVbyfOk=",
        version = "v0.0.0-20151124214156-6c035326b43f",
    )
    go_repository(
        name = "com_github_martini_contrib_render",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/martini-contrib/render",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YFh+sjyJTMQSYjKwM4dFKhJPJC/wfo98tPUc17HdoYw=",
        version = "v0.0.0-20150707142108-ec18f8345a11",
    )
    go_repository(
        name = "com_github_maruel_panicparse_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/maruel/panicparse/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4Gu/Z5oLpJCE/0/NwxrUkyn7alpqOQdJAUuchB2OoJU=",
        version = "v2.2.2",
    )
    go_repository(
        name = "com_github_marusama_semaphore",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/marusama/semaphore",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sq+a5mb8zHbmHhrIH06oqIMGsanjpbxNgxEgZVfgpvQ=",
        version = "v0.0.0-20190110074507-6952cef993b2",
    )
    go_repository(
        name = "com_github_masterminds_glide",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/glide",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:M5MOH04TyRiMBVeWHbifqTpnauxWINIubTCOkhXh+2g=",
        version = "v0.13.2",
    )
    go_repository(
        name = "com_github_masterminds_goutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/goutils",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zukEsf/1JZwCMgHiK3GZftabmxiCw4apj3a28RPBiVg=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_masterminds_semver",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/semver",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:H65muMkzWKEuNDnfl9d70GUjFniHKHRbFPGBuZ3QEww=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_masterminds_semver_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/semver/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hLg3sBzpNErnxhQtUy/mmLR2I9foDujNK030IGemrRc=",
        version = "v3.1.1",
    )
    go_repository(
        name = "com_github_masterminds_sprig",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/sprig",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:z4yfnGrZ7netVz+0EDJ0Wi+5VZCSYp4Z0m2dk6cEM60=",
        version = "v2.22.0+incompatible",
    )
    go_repository(
        name = "com_github_masterminds_vcs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Masterminds/vcs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:USF5TvZGYgIpcbNAEMLfFhHqP08tFZVlUVrmTSpqnyA=",
        version = "v1.13.0",
    )
    go_repository(
        name = "com_github_matryer_is",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/matryer/is",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sosSmIWwkYITGrxZ25ULNDeKiMNzFSr4V/eqBQP0PeE=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_matryer_moq",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/matryer/moq",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HvFwW+cm9bCbZ/+vuGNq7CRWXql8c0y8nGeYpqmpvmk=",
        version = "v0.0.0-20190312154309-6cfb0558e1bd",
    )
    go_repository(
        name = "com_github_mattn_go_colorable",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-colorable",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fFA4WZxdEF4tXPZVKMLwD8oUnCTTo08duU7wxecdEvA=",
        version = "v0.1.13",
    )
    go_repository(
        name = "com_github_mattn_go_ieproxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-ieproxy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HfxbT6/JcvIljmERptWhwa8XzP7H3T+Z2N26gTsaDaA=",
        version = "v0.0.0-20190610004146-91bb50d98149",
    )
    go_repository(
        name = "com_github_mattn_go_isatty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-isatty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BTarxUcIeDqL27Mc+vyvdWYSL28zpIhv3RoTdsLMPng=",
        version = "v0.0.17",
    )
    go_repository(
        name = "com_github_mattn_go_localereader",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-localereader",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ygSAOl7ZXTx4RdPYinUpg6W99U8jWvWi9Ye2JC/oIi4=",
        version = "v0.0.1",
    )
    go_repository(
        name = "com_github_mattn_go_runewidth",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-runewidth",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+xnbZSEeDbOIg5/mE6JF0w6n9duR1l3/WmbinWVwUuU=",
        version = "v0.0.14",
    )
    go_repository(
        name = "com_github_mattn_go_shellwords",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-shellwords",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:K/VxK7SZ+cvuPgFSLKi5QPI9Vr/ipOf4C1gN+ntueUk=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_mattn_go_sqlite3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-sqlite3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qZgc/Rwetq+MtyE18WhzjokPD93dNqLGNT3QJuLvBGw=",
        version = "v1.14.14",
    )
    go_repository(
        name = "com_github_mattn_go_tty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-tty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:d8RFOZ2IiFtFWBcKEHAFYJcPTf0wY5q0exFNJZVWa1U=",
        version = "v0.0.0-20180907095812-13ff1204f104",
    )
    go_repository(
        name = "com_github_mattn_go_zglob",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/go-zglob",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6Ry4EYsScDyt5di4OI6xw1bYhOqfE5S33Z1OPy+d+To=",
        version = "v0.0.3",
    )
    go_repository(
        name = "com_github_mattn_goveralls",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mattn/goveralls",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7eJB6EqsPhRVxvwEXGnqdO2sJI0PTsrWoTMXEk9/OQc=",
        version = "v0.0.2",
    )
    go_repository(
        name = "com_github_matttproud_golang_protobuf_extensions",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/matttproud/golang_protobuf_extensions",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mmDVorXM7PCGKw94cs5zkfA9PSy5pEvNWRP0ET0TIVo=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_mediocregopher_radix_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mediocregopher/radix/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:galbPBjIwmyREgwGCfQEN4X8lxbJnKBYurgz+VfcStA=",
        version = "v3.4.2",
    )
    go_repository(
        name = "com_github_mgutz_ansi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mgutz/ansi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5PJl274Y63IEHC+7izoQE9x6ikvDFZS2mDVS3drnohI=",
        version = "v0.0.0-20200706080929-d51e80ef957d",
    )
    go_repository(
        name = "com_github_mibk_dupl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mibk/dupl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:aZc3jqrF9n0tUHwHt/+jsRxA8cRgA0Gdl56M7W7PoqE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_michaeltjones_walk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/MichaelTJones/walk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FSsoaa1q4jAaeiAUxf9H0PgFP7eA/UL6c3PdJH+nMN4=",
        version = "v0.0.0-20161122175330-4748e29d5718",
    )
    go_repository(
        name = "com_github_microcosm_cc_bluemonday",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/microcosm-cc/bluemonday",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:SMZe2IGa0NuHvnVNAZ+6B38gsTbi5e4sViiWJyDDqFY=",
        version = "v1.0.23",
    )
    go_repository(
        name = "com_github_microsoft_go_winio",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Microsoft/go-winio",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:a9IhgEQBCUEk6QCdml9CiJGhAws+YwffDHEMp1VMrpA=",
        version = "v0.5.2",
    )
    go_repository(
        name = "com_github_microsoft_hcsshim",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Microsoft/hcsshim",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cYnKADiM1869gvBpos3YCteeT6sZLB48lB5dmMMs8Tg=",
        version = "v0.8.18",
    )
    go_repository(
        name = "com_github_microsoft_hcsshim_test",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Microsoft/hcsshim/test",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4FA+QBaydEHlwxg0lMN3rhwoDaQy6LKhVWR4qvq4BuA=",
        version = "v0.0.0-20210227013316-43a75bb4edd3",
    )
    go_repository(
        name = "com_github_miekg_dns",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/miekg/dns",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JKfpVSCB84vrAmHzyrsxB5NAr5kLoMXZArPSw7Qlgyg=",
        version = "v1.1.43",
    )
    go_repository(
        name = "com_github_miekg_pkcs11",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/miekg/pkcs11",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iMwmD7I5225wv84WxIG/bmxz9AXjWvTWIbM/TYHvWtw=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_mileusna_useragent",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mileusna/useragent",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pXqZHmHOz6LN+zbbUgqyGgAWRnnZEI40IzG3tMsXcSI=",
        version = "v0.0.0-20190129205925-3e331f0949a5",
    )
    go_repository(
        name = "com_github_minio_md5_simd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/md5-simd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Gdi1DZK69+ZVMoNHRXJyNcxrMA4dSxoYHZSQbirFg34=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_minio_minio_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/minio-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jw16EimP5oAEM/2wt+SiEUov/YDyTCTDuPtIKgQIvk0=",
        version = "v0.0.0-20190131015406-c8a261de75c1",
    )
    go_repository(
        name = "com_github_minio_minio_go_v7",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/minio-go/v7",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xrc4BQr1Fa4s5RwY0xfMjPZFJ1bcYBCCHYlngBdWV+k=",
        version = "v7.0.21",
    )
    go_repository(
        name = "com_github_minio_sha256_simd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/minio/sha256-simd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:v1ta+49hkWZyvaKwrQB8elexRqm6Y0aMLjCNsrYxo6g=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mistifyio_go_zfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mistifyio/go-zfs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:aKW/4cBs+yK6gpqU3K/oIwk9Q/XICqd3zOX/UFuvqmk=",
        version = "v2.1.2-0.20190413222219-f784269be439+incompatible",
    )
    go_repository(
        name = "com_github_mitchellh_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/cli",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tEElEatulEHDeedTxwckzyYMA5c86fbmNIUL1hBIiTg=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_mitchellh_copystructure",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/copystructure",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Laisrj+bAB6b/yJwB5Bt3ITZhGJdqmxquMKeZ+mmkFQ=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mitchellh_go_homedir",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/go-homedir",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lukF9ziXFxDFPkA1vsr5zpc1XuPDn/wFntq5mG+4E0Y=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_mitchellh_go_ps",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/go-ps",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i6ampVEEF4wQFF+bkYfwYgY+F/uYJDktmvLPf7qIgjc=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mitchellh_go_testing_interface",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/go-testing-interface",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/x0XQ6h+3U3nAyk1yx+bHPURrKa9sVVvYbuqZ7pIAtI=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_github_mitchellh_go_wordwrap",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/go-wordwrap",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6GlHJ/LTGMrIJbwgdqdl2eEH8o+Exx/0m8ir9Gns0u4=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mitchellh_gox",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/gox",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lfGJxY7ToLJQjHHwi0EX6uYBdK78egf954SQl13PQJc=",
        version = "v0.4.0",
    )
    go_repository(
        name = "com_github_mitchellh_iochan",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/iochan",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:C+X3KsSTLFVBr/tK1eYN/vs4rJcvsiLU338UhYPJWeY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mitchellh_mapstructure",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/mapstructure",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jeMsZIYE/09sWLaz43PL7Gy6RuMjD2eJVyuac5Z2hdY=",
        version = "v1.5.0",
    )
    go_repository(
        name = "com_github_mitchellh_osext",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/osext",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2+myh5ml7lgEU/51gbeLHfKGNfgEQQIWrlbdaOsidbQ=",
        version = "v0.0.0-20151018003038-5e2d6d41470f",
    )
    go_repository(
        name = "com_github_mitchellh_reflectwalk",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mitchellh/reflectwalk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9D+8oIskB4VJBN5SFlmc27fSlIBZaov1Wpk/IfikLNY=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mkungla_bexp_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mkungla/bexp/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UqcWAaxWn+rmJ+3ZgwokMrUerGMUeOFihUPrTLPFZ9Q=",
        version = "v3.0.1",
    )
    go_repository(
        name = "com_github_mmatczuk_go_generics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mmatczuk/go_generics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QjLgi/L+MjxysinrA8KkNZLf3cAhTluBoSXUvFeN144=",
        version = "v0.0.0-20181212143635-0aaa050f9bab",
    )
    go_repository(
        name = "com_github_mmcloughlin_geohash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mmcloughlin/geohash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FihR004p/aE1Sju6gcVq5OLDqGcMnpBY+8moBqIsVOs=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_moby_locker",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/locker",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fOXqR41zeveg4fFODix+1Ch4mj/gT0NE1XJbp/epuBg=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_moby_spdystream",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/spdystream",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cjW1zVyyoiM0T7b6UoySUFqzXMoqRckQtXwGPiBhOM8=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_moby_sys_mountinfo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/sys/mountinfo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2Ks8/r6lopsxWi9m58nlwjaeSzUX9iiL1vj5qB/9ObI=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_moby_sys_symlink",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/sys/symlink",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MTFZ74KtNI6qQQpuBxU+uKCim4WtOMokr03hCfJcazE=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_moby_term",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moby/term",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dcztxKSvZ4Id8iPpHERQBbIJfabdt4wUm5qy3wOL2Zc=",
        version = "v0.0.0-20210619224110-3f7ff695adc6",
    )
    go_repository(
        name = "com_github_modern_go_concurrent",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/modern-go/concurrent",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TRLaZ9cD/w8PVh93nsPXa1VrQ6jlwL5oN8l14QlcNfg=",
        version = "v0.0.0-20180306012644-bacd9c7ef1dd",
    )
    go_repository(
        name = "com_github_modern_go_reflect2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/modern-go/reflect2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xBagoLtFs94CBntxluKeaWgTMpvLxC4ur3nMaC9Gz0M=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_modocache_gover",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/modocache/gover",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8Q0qkMVC/MmWkpIdlvZgcv2o2jrlF6zqVOh7W5YHdMA=",
        version = "v0.0.0-20171022184752-b58185e213c5",
    )
    go_repository(
        name = "com_github_montanaflynn_stats",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/montanaflynn/stats",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Duep6KMIDpY4Yo11iFsvyqJDyfzLF9+sndUKT+v64GQ=",
        version = "v0.6.6",
    )
    go_repository(
        name = "com_github_morikuni_aec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/morikuni/aec",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nP9CBfwrvYnBRgY6qfDQkygYDmYwOilePFkwzv4dU8A=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mostynb_go_grpc_compression",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mostynb/go-grpc-compression",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kx3qBtR7GqfI92TJ06DYMCyWCeOkZo2JZyaK6LTRojQ=",
        version = "v1.1.12",
    )
    go_repository(
        name = "com_github_moul_http2curl",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/moul/http2curl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dRMWoAtb+ePxMlLkrCbAqh4TlPHXvoGUSQ323/9Zahs=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_mozillazg_go_slugify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mozillazg/go-slugify",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:SIhqDlnJWZH8OdiTmQgeXR28AOnypmAXPeOTcG7b9lk=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_mozillazg_go_unidecode",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mozillazg/go-unidecode",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vFGEzAH9KSwyWmXCOblazEWDh7fOkpmy/Z4ArmamSUc=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_mrunalp_fileutils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mrunalp/fileutils",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:NKzVxiH7eSk+OQ4M+ZYW1K6h27RUV3MI6NUTsHhU6Z4=",
        version = "v0.5.0",
    )
    go_repository(
        name = "com_github_mschoch_smat",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mschoch/smat",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VeRdUYdCw49yizlSbMEn2SZ+gT+3IUKx8BqxyQdz+BY=",
        version = "v0.0.0-20160514031455-90eadee771ae",
    )
    go_repository(
        name = "com_github_muesli_ansi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/muesli/ansi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kMlmsLSbjkikxQJ1IPwaM+7LJ9ltFu/fi8CRzvSnQmA=",
        version = "v0.0.0-20211031195517-c9f0611b6c70",
    )
    go_repository(
        name = "com_github_muesli_cancelreader",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/muesli/cancelreader",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3I4Kt4BQjOR54NavqnDogx/MIoWBFa0StPA8ELUXHmA=",
        version = "v0.2.2",
    )
    go_repository(
        name = "com_github_muesli_reflow",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/muesli/reflow",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IFsN6K9NfGtjeggFP+68I4chLZV2yIKsXJFNZ+eWh6s=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_muesli_termenv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/muesli/termenv",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wK20DRpJdDX8b7Ek2QfhvqhRQFZ237RGRO0RQ/Iqdy0=",
        version = "v0.13.0",
    )
    go_repository(
        name = "com_github_munnerz_goautoneg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/munnerz/goautoneg",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:C3w9PqII01/Oq1c1nUAm88MOHcQC9l5mIlSMApZMrHA=",
        version = "v0.0.0-20191010083416-a7dc8b61c822",
    )
    go_repository(
        name = "com_github_mwitkow_go_conntrack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mwitkow/go-conntrack",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KUppIJq7/+SVif2QVs3tOP0zanoHgBEVAwHxUSIzRqU=",
        version = "v0.0.0-20190716064945-2f068394615f",
    )
    go_repository(
        name = "com_github_mwitkow_go_proto_validators",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mwitkow/go-proto-validators",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:28i1IjGcx8AofiB4N3q5Yls55VEaitzuEPkFJEVgGkA=",
        version = "v0.0.0-20180403085117-0950a7990007",
    )
    go_repository(
        name = "com_github_mxk_go_flowrate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/mxk/go-flowrate",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:y5//uYreIhSUg3J1GEMiLbxo1LJaP8RfCpH6pymGZus=",
        version = "v0.0.0-20140419014527-cca7078d478f",
    )
    go_repository(
        name = "com_github_nats_io_jwt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nats-io/jwt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+RB5hMpXUUA2dfxuhBTEkMOrYmM+gKIZYS1KjSostMI=",
        version = "v0.3.2",
    )
    go_repository(
        name = "com_github_nats_io_nats_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nats-io/nats.go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ik3HbLhZ0YABLto7iX80pZLPw/6dx3T+++MZJwLnMrQ=",
        version = "v1.9.1",
    )
    go_repository(
        name = "com_github_nats_io_nats_server_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nats-io/nats-server/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i2Ly0B+1+rzNZHHWtD4ZwKi+OU5l+uQo1iDHZ2PmiIc=",
        version = "v2.1.2",
    )
    go_repository(
        name = "com_github_nats_io_nkeys",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nats-io/nkeys",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6JrEfig+HzTH85yxzhSVbjHRJv9cn0p6n3IngIcM5/k=",
        version = "v0.1.3",
    )
    go_repository(
        name = "com_github_nats_io_nuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nats-io/nuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5iA8DT8V7q8WK2EScv2padNa/rTESc1KdnPw4TC2paw=",
        version = "v1.0.1",
    )
    go_repository(
        name = "com_github_ncw_swift",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ncw/swift",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4DQRPj35Y41WogBxyhOXlrI37nzGlyEcsforeudyYPQ=",
        version = "v1.0.47",
    )
    go_repository(
        name = "com_github_ngdinhtoan_glide_cleanup",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ngdinhtoan/glide-cleanup",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kN4sV+0tp2F1BvwU+5SfNRMDndRmvIfnI3kZ7B8Yv4Y=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_niemeyer_pretty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/niemeyer/pretty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fD57ERR4JtEqsWbfPhv4DMiApHyliiK5xCTNVSPiaAs=",
        version = "v0.0.0-20200227124842-a10e7caefd8e",
    )
    go_repository(
        name = "com_github_nightlyone_lockfile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nightlyone/lockfile",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RHep2cFKK4PonZJDdEl4GmkabuhbsRMgk/k3uAmxBiA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_nishanths_predeclared",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nishanths/predeclared",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3f0nxAmdj/VoCGN/ijdMy7bj6SBagaqYg1B0hu8clMA=",
        version = "v0.0.0-20200524104333-86fad755b4d3",
    )
    go_repository(
        name = "com_github_nkovacs_streamquote",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nkovacs/streamquote",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:E2B8qYyeSgv5MXpmzZXRNp8IAQ4vjxIjhpAf5hv/tAg=",
        version = "v0.0.0-20170412213628-49af9bddb229",
    )
    go_repository(
        name = "com_github_nvveen_gotty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Nvveen/Gotty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TngWCqHvy9oXAN6lEVMRuU21PR1EtLVZJmdB18Gu3Rw=",
        version = "v0.0.0-20120604004816-cd527374f1e5",
    )
    go_repository(
        name = "com_github_nxadm_tail",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/nxadm/tail",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DQuhQpB1tVlglWS2hLQ5OV6B5r8aGxSrPc5Qo6uTN78=",
        version = "v1.4.4",
    )
    go_repository(
        name = "com_github_nytimes_gziphandler",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/NYTimes/gziphandler",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lsxEuwrXEAokXB9qhlbKWPpo3KMLZQ5WB5WLQRW1uq0=",
        version = "v0.0.0-20170623195520-56545f4a5d46",
    )
    go_repository(
        name = "com_github_oklog_oklog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oklog/oklog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wVfs8F+in6nTBMkA7CbRw+zZMIB7nNM825cM1wuzoTk=",
        version = "v0.3.2",
    )
    go_repository(
        name = "com_github_oklog_run",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oklog/run",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GEenZ1cK0+q0+wsJew9qUg/DyD8k3JzYsZAi5gYi2mA=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_oklog_ulid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oklog/ulid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EGfNDEx6MqHz8B3uNV6QAib1UR2Lm97sHi3ocA6ESJ4=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_olekukonko_tablewriter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/olekukonko/tablewriter",
        replace = "github.com/cockroachdb/tablewriter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:c7yLgqcm/3c9lYtpWeVD9NYqA9cKsKHdpQM62PHtTUM=",
        version = "v0.0.5-0.20200105123400-bd15540e8847",
    )
    go_repository(
        name = "com_github_oneofone_xxhash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/OneOfOne/xxhash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KMrpdQIwFcEqXDklaen+P1axHaj9BSKzvpUUfnHldSE=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_onsi_ginkgo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/onsi/ginkgo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2mOpI4JVVPBN+WQRa0WKH2eXR+Ey+uK4n7Zj0aYpIQA=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_github_onsi_gomega",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/onsi/gomega",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gph6h/qe9GSUw1NhH1gp+qb+h8rXD8Cy60Z32Qw3ELA=",
        version = "v1.10.3",
    )
    go_repository(
        name = "com_github_op_go_logging",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/op/go-logging",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lDH9UUVJtmYCjyT0CI4q8xvlXPxeZ0gYCVvWbmPlp88=",
        version = "v0.0.0-20160315200505-970db520ece7",
    )
    go_repository(
        name = "com_github_opencontainers_go_digest",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/go-digest",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:apOUWs51W5PlhuyGyz9FCeeBIOUDA/6nW8Oi/yOhh5U=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_opencontainers_image_spec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/image-spec",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9yCKha/T5XdGtO0q9Q9a6T5NUCsTn/DrBg0D7ufOcFM=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_opencontainers_runc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/runc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vIXrkId+0/J2Ymu2m7VjGvbSlAId9XNRPhn2p4b+d8w=",
        version = "v1.1.3",
    )
    go_repository(
        name = "com_github_opencontainers_runtime_spec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/runtime-spec",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3snG66yBm59tKhhSPQrQ/0bCrv1LQbKt40LnUPiUxdc=",
        version = "v1.0.3-0.20210326190908-1c3f411f0417",
    )
    go_repository(
        name = "com_github_opencontainers_runtime_tools",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/runtime-tools",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:H7DMc6FAjgwZZi8BRqjrAAHWoqEr5e5L6pS4V0ezet4=",
        version = "v0.0.0-20181011054405-1d69bd0f9c39",
    )
    go_repository(
        name = "com_github_opencontainers_selinux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opencontainers/selinux",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rAiKF8hTcgLI3w0DHm6i0ylVVcOrlgR1kK99DRLDhyU=",
        version = "v1.10.0",
    )
    go_repository(
        name = "com_github_opennota_wd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/opennota/wd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cVQhwfBgiKTMAdYPbVeuIiTkdY59qZ3sp5RpyO8CNtg=",
        version = "v0.0.0-20180911144301-b446539ab1e7",
    )
    go_repository(
        name = "com_github_ory_dockertest_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ory/dockertest/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:v4dkG+dlu76goxMiTT2j8zV7s4oPPEppKT8K8p2f1kY=",
        version = "v3.9.1",
    )
    go_repository(
        name = "com_github_otan_gopgkrb5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/otan/gopgkrb5",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iDZlYPC8mxvKvpIvDu66j48Q4WqW15HuWUX+MIwjF0U=",
        version = "v1.0.3",
    )
    go_repository(
        name = "com_github_oxtoacart_bpool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/oxtoacart/bpool",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rp5dCmg/yLR3mgFuSOe4oEnDDmGLROTvMragMUXpTQw=",
        version = "v0.0.0-20190530202638-03653db5a59c",
    )
    go_repository(
        name = "com_github_pact_foundation_pact_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pact-foundation/pact-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OYkFijGHoZAYbOIb1LWXrwKQbMMRUv1oQ89blD2Mh2Q=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_pascaldekloe_goe",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pascaldekloe/goe",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cBOtyMzM9HTpWjXfbbunk26uA6nG3a8n06Wieeh0MwY=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_patrickmn_go_cache",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/patrickmn/go-cache",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HRMgzkcYKYpi3C8ajMPV8OFXaaRUnok+kx1WdO15EQc=",
        version = "v2.1.0+incompatible",
    )
    go_repository(
        name = "com_github_paulbellamy_ratecounter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/paulbellamy/ratecounter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2L/RhJq+HA8gBQImDXtLPrDXK5qAj6ozWVK/zFXVJGs=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_pborman_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pborman/uuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:J7Q5mO4ysT1dv8hyrUGHb9+ooztCXu1D8MY8DZYsu3g=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_pelletier_go_toml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pelletier/go-toml",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zeC5b1GviRUyKYd6OJPvBU/mcVDVoL1OhT17FCt5dSQ=",
        version = "v1.9.3",
    )
    go_repository(
        name = "com_github_pelletier_go_toml_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pelletier/go-toml/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ipoSadvV8oGUjnUbMub59IDPPwfxF694nG/jwbMiyQg=",
        version = "v2.0.5",
    )
    go_repository(
        name = "com_github_performancecopilot_speed",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/performancecopilot/speed",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2WnRzIquHa5QxaJKShDkLM+sc0JPuwhXzK8OYOyt3Vg=",
        version = "v3.0.0+incompatible",
    )
    go_repository(
        name = "com_github_peterbourgon_diskv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/peterbourgon/diskv",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UBdAOUP5p4RWqPBg048CAvpKN+vxiaj6gdUUzhl4XmI=",
        version = "v2.0.1+incompatible",
    )
    go_repository(
        name = "com_github_peterbourgon_g2s",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/peterbourgon/g2s",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sKwxy1H95npauwu8vtF95vG/syrL0p8fSZo/XlDg5gk=",
        version = "v0.0.0-20170223122336-d4e7ad98afea",
    )
    go_repository(
        name = "com_github_peterh_liner",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/peterh/liner",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:O62NGAXV0cNzBI6e7vI3zTHSTgPHsWIcS3Q4XC1/pAU=",
        version = "v1.0.1-0.20180619022028-8c1271fcf47f",
    )
    go_repository(
        name = "com_github_petermattis_goid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/petermattis/goid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:64bxqeTEN0/xoEqhKGowgihNuzISS9rEG6YUMU4bzJo=",
        version = "v0.0.0-20211229010228-4d14c490ee36",
    )
    go_repository(
        name = "com_github_philhofer_fwd",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/philhofer/fwd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UbZqGr5Y38ApvM/V/jEljVxwocdweyH+vmYvRPBnbqQ=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_pierrec_lz4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pierrec/lz4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WCjObylUIOlKy/+7Abdn34TLIkXiA4UWUMhxq9m9ZXI=",
        version = "v2.5.2+incompatible",
    )
    go_repository(
        name = "com_github_pierrec_lz4_v4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pierrec/lz4/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kV4Ip+/hUBC+8T6+2EgburRtkE9ef4nbY3f4dFhGjMc=",
        version = "v4.1.17",
    )
    go_repository(
        name = "com_github_pierrre_compare",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pierrre/compare",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:k4IUsHgh+dbcAOIWCfxVa/7G6STjADH2qmhomv+1quc=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_pierrre_geohash",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pierrre/geohash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:f/zfjdV4rVofTCz1FhP07T+EMQAvcMM2ioGZVt+zqjI=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_pingcap_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pingcap/errors",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lFuQV/oaUMGcD2tqt+01ROSmJs75VG1ToEOkZIZ4nE4=",
        version = "v0.11.4",
    )
    go_repository(
        name = "com_github_pires_go_proxyproto",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pires/go-proxyproto",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IukmRewDQFWC7kfnb66CSomk2q/seBuilHBYFwyq0Hs=",
        version = "v0.7.0",
    )
    go_repository(
        name = "com_github_pkg_browser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/browser",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Qj1ukM4GlMWXNdMBuXcXfz/Kw9s1qm0CLY32QxuSImI=",
        version = "v0.0.0-20210115035449-ce105d075bb4",
    )
    go_repository(
        name = "com_github_pkg_diff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/diff",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:aoZm08cpOy4WuID//EZDgcC4zIxODThtZNPirFr42+A=",
        version = "v0.0.0-20210226163009-20ebb0f2a09e",
    )
    go_repository(
        name = "com_github_pkg_errors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/errors",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=",
        version = "v0.9.1",
    )
    go_repository(
        name = "com_github_pkg_profile",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/profile",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hUDfIISABYI59DyeB3OTay/HxSRwTQ8rB/H83k6r5dM=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_pkg_sftp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/sftp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:I2qBYMChEhIjOgazfJmV3/mZM256btk6wkCDRmW7JYs=",
        version = "v1.13.1",
    )
    go_repository(
        name = "com_github_pkg_term",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pkg/term",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tFwafIEMf0B7NlcxV/zJ6leBIa81D3hgGSgsE5hCkOQ=",
        version = "v0.0.0-20180730021639-bffc007b7fd5",
    )
    go_repository(
        name = "com_github_planetscale_vtprotobuf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/planetscale/vtprotobuf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GFCKgmp0tecUJ0sJuv4pzYCqS9+RGSn52M3FUwPs+uo=",
        version = "v0.6.1-0.20240319094008-0393e58bdf10",
    )
    go_repository(
        name = "com_github_pmezard_go_difflib",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pmezard/go-difflib",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Jamvg5psRIccs7FGNTlIRMkT8wgtp5eCXdBlqhYGL6U=",
        version = "v1.0.1-0.20181226105442-5d4384ee4fb2",
    )
    go_repository(
        name = "com_github_posener_complete",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/posener/complete",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:NP0eAhjcjImqslEwo/1hq7gpajME0fTLTezBKDqfXqo=",
        version = "v1.2.3",
    )
    go_repository(
        name = "com_github_power_devops_perfstat",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/power-devops/perfstat",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ncq/mPwQF4JjgDlrVEn3C11VoGHZN7m8qihwgMEtzYw=",
        version = "v0.0.0-20210106213030-5aafc221ea8c",
    )
    go_repository(
        name = "com_github_pquerna_cachecontrol",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pquerna/cachecontrol",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BLqxdwZ6j771IpSCRx7s/GJjXHUE00Hmu7/YegCGdzA=",
        version = "v0.0.0-20200921180117-858c6e7e6b7e",
    )
    go_repository(
        name = "com_github_pressly_goose_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pressly/goose/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lIQIIXVbdO2RuQtJBS1e7MZjKEk0demVWt6i0YPiOrg=",
        version = "v3.5.3",
    )
    go_repository(
        name = "com_github_prometheus_alertmanager",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/alertmanager",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KIb9IChC3kg+1CC388qfr7bsT+tARpQqdsCMoatdObA=",
        version = "v0.23.0",
    )
    go_repository(
        name = "com_github_prometheus_client_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/client_golang",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5fCgGYogn0hFdhyhLbw7hEsWxufKtY9klyvdNfFlFhM=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_github_prometheus_client_model",
        build_directives = [
            "gazelle:resolve go go github.com/golang/protobuf/ptypes/timestamp @com_github_golang_protobuf//ptypes/timestamp:go_default_library",
        ],
        build_file_proto_mode = "default",
        importpath = "github.com/prometheus/client_model",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_prometheus_client_model.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UBgGFHqYdG/TPFD1B1ogZywDqEkwp3fBMvqdiQ7Xew4=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_prometheus_common",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/common",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EKsfXEYo4JpWMHH5cg+KOUWeuJSov1Id8zGR8eeI1YM=",
        version = "v0.42.0",
    )
    go_repository(
        name = "com_github_prometheus_common_sigv4",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/common/sigv4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qoVebwtwwEhS85Czm2dSROY5fTo2PAPEVdDeppTwGX4=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_prometheus_exporter_toolkit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/exporter-toolkit",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Aqk75wQD92N9CqmTlZwjKwq6272nOGrWIbc8Z7+xQO0=",
        version = "v0.6.1",
    )
    go_repository(
        name = "com_github_prometheus_procfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/procfs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wzCHvIvM5SxWqYvwgVL7yJY8Lz3PKn49KQtpgMYJfhI=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_prometheus_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/prometheus",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:U8ZpFGP11pZi0ZavWWGeAqck3dVT9AY6zbr4fbBftjA=",
        version = "v1.8.2-0.20210914090109-37468d88dce8",
    )
    go_repository(
        name = "com_github_prometheus_statsd_exporter",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/statsd_exporter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hA05Q5RFeIjgwKIYEdFd59xu5Wwaznf33yKI+pyX6T8=",
        version = "v0.21.0",
    )
    go_repository(
        name = "com_github_prometheus_tsdb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/prometheus/tsdb",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YZcsG11NqnK4czYLrWd9mpEuAJIHVQLwdrleYfszMAA=",
        version = "v0.7.1",
    )
    go_repository(
        name = "com_github_pseudomuto_protoc_gen_doc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pseudomuto/protoc-gen-doc",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:com_github_pseudomuto_protoc_gen_doc.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:61vWZuxYa8D7Rn4h+2dgoTNqnluBmJya2MgbqO32z6g=",
        version = "v1.3.2",
    )
    go_repository(
        name = "com_github_pseudomuto_protokit",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/pseudomuto/protokit",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hlnBDcy3YEDXH7kc9gV+NLaN0cDzhDvD1s7Y6FZ8RpM=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_puerkitobio_goquery",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/PuerkitoBio/goquery",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:PSPBGne8NIUWw+/7vFBV+kG2J/5MOjbzc7154OaKCSE=",
        version = "v1.5.1",
    )
    go_repository(
        name = "com_github_puerkitobio_purell",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/PuerkitoBio/purell",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WEQqlqaGbrPkxLJWfBwQmfEAE1Z7ONdDLqrN38tNFfI=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_puerkitobio_urlesc",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/PuerkitoBio/urlesc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:d+Bc7a5rLufV/sSk/8dngufqelfh6jnri85riMAaF/M=",
        version = "v0.0.0-20170810143723-de5bf2ad4578",
    )
    go_repository(
        name = "com_github_rcrowley_go_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rcrowley/go-metrics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:N/ElC8H3+5XpJzTSTfLsJV/mx9Q9g7kxmchpfZyxgzM=",
        version = "v0.0.0-20201227073835-cf1acfcdf475",
    )
    go_repository(
        name = "com_github_remyoudompheng_bigfft",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/remyoudompheng/bigfft",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OdAsTTz6OkFY5QxjkYwrChwuRruF69c169dPK26NUlk=",
        version = "v0.0.0-20200410134404-eec4a21b6bb0",
    )
    go_repository(
        name = "com_github_retailnext_hllpp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/retailnext/hllpp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RnWNS9Hlm8BIkjr6wx8li5abe0fr73jljLycdfemTp0=",
        version = "v1.0.1-0.20180308014038-101a6d2f8b52",
    )
    go_repository(
        name = "com_github_rivo_uniseg",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rivo/uniseg",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:S1pD9weZBuJdFmowNwbpi7BJ8TNftyUImj/0WQi72jY=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_robertkrimen_godocdown",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/robertkrimen/godocdown",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jMxcLa+VjJKhpCwbLUXAD15wJ+hhvXMLujCl3MkXpfM=",
        version = "v0.0.0-20130622164427-0bfa04905481",
    )
    go_repository(
        name = "com_github_robfig_cron_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/robfig/cron/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WdRxkvbJztn8LMz/QEvLN5sBU+xKpSqwwUO1Pjr4qDs=",
        version = "v3.0.1",
    )
    go_repository(
        name = "com_github_rogpeppe_fastuuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rogpeppe/fastuuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Ppwyp6VYCF1nvBTXL3trRso7mXMlRrw9ooo375wvi2s=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_rogpeppe_go_internal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rogpeppe/go-internal",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UQB4HGPB6osV0SQTLymcB4TgvyWu6ZyliaW0tI/otEQ=",
        version = "v1.14.1",
    )
    go_repository(
        name = "com_github_rs_cors",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rs/cors",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:P2KMzcFwrPoSjkF1WLRPsp3UMLyql8L4v9hQpVeK5so=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_github_rs_xid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rs/xid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6NjYksEUlhurdVehpc7S7dk6DAmcKv8V9gG0FsVN2U4=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_github_rs_zerolog",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/rs/zerolog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uPRuwkWF4J6fGsJ2R0Gn2jB1EQiav9k3S6CSdygQJXY=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_github_russross_blackfriday",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/russross/blackfriday",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KqfZb0pUVN2lYqZUYRddxF4OR8ZMURnJIG5Y3VRLtww=",
        version = "v1.6.0",
    )
    go_repository(
        name = "com_github_russross_blackfriday_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/russross/blackfriday/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JIOH55/0cWyOuilr9/qlrm0BSXldqnqwMsf35Ld67mk=",
        version = "v2.1.0",
    )
    go_repository(
        name = "com_github_ryanuber_columnize",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ryanuber/columnize",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:j1Wcmh8OrK4Q7GXY+V7SVSY8nUWQxHW5TkBe7YUl+2s=",
        version = "v2.1.0+incompatible",
    )
    go_repository(
        name = "com_github_safchain_ethtool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/safchain/ethtool",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2c1EFnZHIPCW8qKWgHMH/fX2PkSabFc5mrVzfUNdg5U=",
        version = "v0.0.0-20190326074333-42ed695e3de8",
    )
    go_repository(
        name = "com_github_sahilm_fuzzy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sahilm/fuzzy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FzWGaw2Opqyu+794ZQ9SYifWv2EIXpwP4q8dY1kDAwI=",
        version = "v0.1.0",
    )
    go_repository(
        name = "com_github_samuel_go_zookeeper",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/samuel/go-zookeeper",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CGjiMQ0wMH4wtNWrlj6kiTbkPt2F3rbYnhGX6TWLfco=",
        version = "v0.0.0-20200724154423-2164a8ac840e",
    )
    go_repository(
        name = "com_github_sap_go_hdb",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/SAP/go-hdb",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hkw4ozGZ/i4eak7ZuGkY5e0hxiXFdNUBNhr4AvZVNFE=",
        version = "v0.14.1",
    )
    go_repository(
        name = "com_github_sasha_s_go_deadlock",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sasha-s/go-deadlock",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sqv7fDNShgjcaxkO0JNcOAlr8B9+cV5Ey/OB71efZx0=",
        version = "v0.3.1",
    )
    go_repository(
        name = "com_github_satori_go_uuid",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/satori/go.uuid",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gQZ0qzfKHQIybLANtM3mBXNUtOfsCFXeTsnBqCsx1KM=",
        version = "v1.2.1-0.20181028125025-b2ce2384e17b",
    )
    go_repository(
        name = "com_github_savsgio_gotils",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/savsgio/gotils",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ocK/D6lCgLji37Z2so4xhMl46se1ntReQQCUIU4BWI8=",
        version = "v0.0.0-20210921075833-21a6215cb0e4",
    )
    go_repository(
        name = "com_github_scaleway_scaleway_sdk_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/scaleway/scaleway-sdk-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3egqo0Vut6daANFm7tOXdNAa8v5/uLU+sgCJrc88Meo=",
        version = "v1.0.0-beta.7.0.20210223165440-c65ae3540d44",
    )
    go_repository(
        name = "com_github_schollz_closestmatch",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/schollz/closestmatch",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Uel2GXEpJqOWBrlyI+oY9LTiyyjYS17cCYRqP13/SHk=",
        version = "v2.1.0+incompatible",
    )
    go_repository(
        name = "com_github_scylladb_termtables",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/scylladb/termtables",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8qmTC5ByIXO3GP/IzBkxcZ/99VITvnIETDhdFz/om7A=",
        version = "v0.0.0-20191203121021-c4c0b6d42ff4",
    )
    go_repository(
        name = "com_github_sean__seed",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sean-/seed",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nn5Wsu0esKSJiIVhscUtVbo7ada43DJhG55ua/hjS5I=",
        version = "v0.0.0-20170313163322-e2103e2c3529",
    )
    go_repository(
        name = "com_github_seccomp_libseccomp_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/seccomp/libseccomp-golang",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RpforrEYXWkmGwJHIGnLZ3tTWStkjVVstwzNGqxX2Ds=",
        version = "v0.9.2-0.20220502022130-f33da4d89646",
    )
    go_repository(
        name = "com_github_segmentio_kafka_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/segmentio/kafka-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HtCSf6B4gN/87yc5qTl7WsxPKQIIGXLPPM1bMCPOsoY=",
        version = "v0.2.0",
    )
    go_repository(
        name = "com_github_sergi_go_diff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sergi/go-diff",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Kpca3qRNrduNnOQeazBd0ysaKrUJiIuISHxogkT9RPQ=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_shirou_gopsutil_v3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shirou/gopsutil/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VoGxEW2hpmz0Vt3wUvHIl9fquzYLNpVpgNNB7pGJimA=",
        version = "v3.21.12",
    )
    go_repository(
        name = "com_github_shopify_goreferrer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Shopify/goreferrer",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KkH3I3sJuOLP3TjA/dfr4NAY8bghDwnXiU7cTKxQqo0=",
        version = "v0.0.0-20220729165902-8cddb4f5de06",
    )
    go_repository(
        name = "com_github_shopify_logrus_bugsnag",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Shopify/logrus-bugsnag",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UrqY+r/OJnIp5u0s1SbQ8dVfLCZJsnvazdBP5hS4iRs=",
        version = "v0.0.0-20171204204709-577dee27f20d",
    )
    go_repository(
        name = "com_github_shopify_sarama",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Shopify/sarama",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lqqPUPQZ7zPqYlWpTh+LQ9bhYNu2xJL6k1SJN4WVe2A=",
        version = "v1.38.1",
    )
    go_repository(
        name = "com_github_shopify_toxiproxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Shopify/toxiproxy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TKdv8HiTLgE5wdJuEML90aBgNWsokNbMijUGhmcoBJc=",
        version = "v2.1.4+incompatible",
    )
    go_repository(
        name = "com_github_shopify_toxiproxy_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/Shopify/toxiproxy/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:i4LPT+qrSlKNtQf5QliVjdP08GyAH8+BUIc9gT0eahc=",
        version = "v2.5.0",
    )
    go_repository(
        name = "com_github_shopspring_decimal",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shopspring/decimal",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:abSATXmQEYyShuxI4/vyW3tV1MrKAJzCZ/0zLUXYbsQ=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_shurcool_httpfs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/httpfs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:bUGsEnyNbVPw06Bs80sCeARAlK8lhwqGyi6UT8ymuGk=",
        version = "v0.0.0-20190707220628-8d4bc4ba7749",
    )
    go_repository(
        name = "com_github_shurcool_sanitized_anchor_name",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/sanitized_anchor_name",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:PdmoCO6wvbs+7yrJyMORt4/BmY5IYyJwS/kOiWx8mHo=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_shurcool_vfsgen",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/shurcooL/vfsgen",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pXY9qYc/MP5zdvqWEUH6SjNiu7VhSjuVFTFiTcphaLU=",
        version = "v0.0.0-20200824052919-0d455de96546",
    )
    go_repository(
        name = "com_github_sirupsen_logrus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sirupsen/logrus",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:trlNQbNUG3OdDrDil03MCb1H2o9nJ1x4/5LYw7byDE0=",
        version = "v1.9.0",
    )
    go_repository(
        name = "com_github_sjmudd_stopwatch",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sjmudd/stopwatch",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:acClJNSOjUrAUKW+ZneCZymCFDWtSaJG5YQl8FoOlyI=",
        version = "v0.0.0-20170613150411-f380bf8a9be1",
    )
    go_repository(
        name = "com_github_slack_go_slack",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/slack-go/slack",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:j7uOUDowybWf9eSgZg/AbGx6J1OPJB6SE8Z5dNl6Mtw=",
        version = "v0.9.5",
    )
    go_repository(
        name = "com_github_slok_go_http_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/slok/go-http-metrics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rh0LaYEKza5eaYRGDXujKrOln57nHBi4TtVhmNEpbgM=",
        version = "v0.10.0",
    )
    go_repository(
        name = "com_github_smartystreets_assertions",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/smartystreets/assertions",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Jpy1PXuP99tXNrhbq2BaPz9B+jNAvH1JPQQpG/9GCXY=",
        version = "v0.0.0-20190116191733-b6c0e53d7304",
    )
    go_repository(
        name = "com_github_smartystreets_goconvey",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/smartystreets/goconvey",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fv0U8FUIMPNf1L9lnHLvLhgicrIVChEkdzIKYqbNC9s=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_github_snowflakedb_gosnowflake",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/snowflakedb/gosnowflake",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Gyoi6g4lMHsilEwW9+KV+bgYkJTgf5pVfvL7Utus920=",
        version = "v1.3.4",
    )
    go_repository(
        name = "com_github_soheilhy_cmux",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/soheilhy/cmux",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0HKaf1o97UwFjHH9o5XsHUOF+tqmdA7KEzXLpiyaw0E=",
        version = "v0.1.4",
    )
    go_repository(
        name = "com_github_sony_gobreaker",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/sony/gobreaker",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oMnRNZXX5j85zso6xCPRNPtmAycat+WcoKbklScLDgQ=",
        version = "v0.4.1",
    )
    go_repository(
        name = "com_github_spaolacci_murmur3",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spaolacci/murmur3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qLC7fQah7D6K1B0ujays3HV9gkFtllcxhzImRR7ArPQ=",
        version = "v0.0.0-20180118202830-f09979ecbc72",
    )
    go_repository(
        name = "com_github_spf13_afero",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/afero",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EaGW2JJh15aKOejeuJ+wpFSHnbd7GE6Wvp3TsNhb6LY=",
        version = "v1.10.0",
    )
    go_repository(
        name = "com_github_spf13_cast",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/cast",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nFm6S0SMdyzrzcmThSipiEubIDy8WEXKNZ0UOgiRpng=",
        version = "v1.3.1",
    )
    go_repository(
        name = "com_github_spf13_cobra",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/cobra",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+KmjbUw1hriSNMF55oPrkZcb27aECyrj8V2ytv7kWDw=",
        version = "v1.2.1",
    )
    go_repository(
        name = "com_github_spf13_jwalterweatherman",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/jwalterweatherman",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ue6voC5bR5F8YxI5S67j9i582FU4Qvo2bmqnqMYADFk=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_spf13_pflag",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/pflag",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iy+VFUOCP1a+8yFto/drg2CJ5u0yRoB7fZw3DKv/JXA=",
        version = "v1.0.5",
    )
    go_repository(
        name = "com_github_spf13_viper",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spf13/viper",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Kq1fyeebqsBfbjZj4EL7gj2IO0mMaiyjYUWcUsl2O44=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_github_spiffe_go_spiffe_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/spiffe/go-spiffe/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:l+DolpxNWYgruGQVV0xsfeya3CsC7m8iBzDnMpsbLuo=",
        version = "v2.6.0",
    )
    go_repository(
        name = "com_github_stefanberger_go_pkcs11uri",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stefanberger/go-pkcs11uri",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lIOOHPEbXzO3vnmx2gok1Tfs31Q8GQqKLc8vVqyQq/I=",
        version = "v0.0.0-20201008174630-78d3cae3a980",
    )
    go_repository(
        name = "com_github_stephens2424_writerset",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stephens2424/writerset",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:znRLgU6g8RS5euYRcy004XeE4W+Tu44kALzy7ghPif8=",
        version = "v1.0.2",
    )
    go_repository(
        name = "com_github_stoewer_go_strcase",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stoewer/go-strcase",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Z2iHWqGXH00XYgqDmNgQbIBxf3wrNq0F3feEy0ainaU=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_streadway_amqp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/streadway/amqp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WhxRHzgeVGETMlmVfqhRn8RIeeNoPr2Czh33I4Zdccw=",
        version = "v0.0.0-20190827072141-edfb9018d271",
    )
    go_repository(
        name = "com_github_streadway_handy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/streadway/handy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AhmOdSHeswKHBjhsLs/7+1voOxT+LLrSk/Nxvk35fug=",
        version = "v0.0.0-20190108123426-d5acb3125c2a",
    )
    go_repository(
        name = "com_github_stretchr_objx",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stretchr/objx",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xuMeJ0Sdp5ZMRXx/aWO6RZxdr3beISkG5/G/aIRr3pY=",
        version = "v0.5.2",
    )
    go_repository(
        name = "com_github_stretchr_testify",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/stretchr/testify",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7s2iGBzp5EwR7/aIZr8ao5+dra3wiQyKjjFuvgVKu7U=",
        version = "v1.11.1",
    )
    go_repository(
        name = "com_github_subosito_gotenv",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/subosito/gotenv",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Slr1R9HxAlEKefgq5jn9U+DnETlIUa6HfgEzj0g5d7s=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_syndtr_gocapability",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/syndtr/gocapability",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kdXcSzyDtseVEc4yCz2qF8ZrQvIDBJLl4S1c3GCXmoI=",
        version = "v0.0.0-20200815063812-42c35b437635",
    )
    go_repository(
        name = "com_github_tchap_go_patricia",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tchap/go-patricia",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JvoDL7JSoIP2HDE8AbDH3zC8QBPxmzYe32HHy5yQ+Ck=",
        version = "v2.2.6+incompatible",
    )
    go_repository(
        name = "com_github_tdewolff_minify_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tdewolff/minify/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kejsHQMM17n6/gwdw53qsi6lg0TGddZADVyQOz1KMdE=",
        version = "v2.12.4",
    )
    go_repository(
        name = "com_github_tdewolff_parse_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tdewolff/parse/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KCkDvNUMof10e3QExio9OPZJT8SbdKojLBumw8YZycQ=",
        version = "v2.6.4",
    )
    go_repository(
        name = "com_github_tebeka_selenium",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tebeka/selenium",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cNziB+etNgyH/7KlNI7RMC1ua5aH1+5wUlFQyzeMh+w=",
        version = "v0.9.9",
    )
    go_repository(
        name = "com_github_the42_cartconvert",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/the42/cartconvert",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:I4DY8wLxJXCrMYzDM6lKCGc3IQwJX0PlTLsd3nQqI3c=",
        version = "v0.0.0-20131203171324-aae784c392b8",
    )
    go_repository(
        name = "com_github_tidwall_pretty",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tidwall/pretty",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HsD+QiTn7sK6flMKIvNmpqz1qrpP3Ps6jOKIKMooyg4=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_tinylib_msgp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tinylib/msgp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TnCZ3FIuKeaIy+F45+Cnp+caqdXGy4z74HvwXN+570Y=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_tklauser_go_sysconf",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tklauser/go-sysconf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JeUVdAOWhhxVcU6Eqr/ATFHgXk/mmiItdKeJPev3vTo=",
        version = "v0.3.9",
    )
    go_repository(
        name = "com_github_tklauser_numcpus",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tklauser/numcpus",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ILuRUQBtssgnxw0XXIjKUC56fgnOrFoQQ/4+DeU2biQ=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_github_tmc_grpc_websocket_proxy",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tmc/grpc-websocket-proxy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LnC5Kc/wtumK+WB441p7ynQJzVuNRJiqddSIE3IlSEQ=",
        version = "v0.0.0-20190109142713-0ad062ec5ee5",
    )
    go_repository(
        name = "com_github_tomihiltunen_geohash_golang",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/TomiHiltunen/geohash-golang",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wumPkzt4zaxO4rHPBrjDK8iZMR41C1qs7njNqlacwQg=",
        version = "v0.0.0-20150112065804-b3e4e625abfb",
    )
    go_repository(
        name = "com_github_toqueteos_webbrowser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/toqueteos/webbrowser",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tVP/gpK69Fx+qMJKsLE7TD8LuGWPnEV71wBN9rrstGQ=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_trivago_tgo",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/trivago/tgo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uaWH/XIy9aWYWpjm2CU3RpcqZXmX2ysQ9/Go+d9gyrM=",
        version = "v1.0.7",
    )
    go_repository(
        name = "com_github_tv42_httpunix",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/tv42/httpunix",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u6SKchux2yDvFQnDHS3lPnIRmfVJ5Sxy3ao2SIdysLQ=",
        version = "v0.0.0-20191220191345-2ba4b9c3382c",
    )
    go_repository(
        name = "com_github_twitchtv_twirp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/twitchtv/twirp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KGXanpa9LXdVE/V5P/tA27rkKFmXRGCtSNT7zdeeVOY=",
        version = "v8.1.0+incompatible",
    )
    go_repository(
        name = "com_github_twpayne_go_geom",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/twpayne/go-geom",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:I2MtC83UDniuTVzGWZOyhMn9hDPrjLm0jsUbohimtZQ=",
        version = "v1.4.2",
    )
    go_repository(
        name = "com_github_twpayne_go_kml",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/twpayne/go-kml",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rFMw2/EwgkVssGS2MT6YfWSPZz6BgcJkLxQ53jnE8rQ=",
        version = "v1.5.2",
    )
    go_repository(
        name = "com_github_twpayne_go_polyline",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/twpayne/go-polyline",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EA8HPg0MNS62R5D2E8B6zyz2TMkmkXVlQaVBVgY3F5A=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_twpayne_go_waypoint",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/twpayne/go-waypoint",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0DvmyIQBIEYpXJt7Wm242Wn6ePH84FR5wv2Jh/yRgUw=",
        version = "v0.0.0-20200706203930-b263a7f6e4e8",
    )
    go_repository(
        name = "com_github_uber_athenadriver",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/uber/athenadriver",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:k6k0RBeXjR7oZ8NO557MsRw3eX1cc/9B0GNx+W9eHiQ=",
        version = "v1.1.4",
    )
    go_repository(
        name = "com_github_uber_go_atomic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/uber-go/atomic",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yOuPqEq4ovnhEjpHmfFwsqBXDYbQeT6Nb0bwD6XnD5o=",
        version = "v1.4.0",
    )
    go_repository(
        name = "com_github_uber_go_tally",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/uber-go/tally",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9hLSgNBP28CjIaDmAuRTq9qV+UZY+9PcvAkXO4nNMwg=",
        version = "v3.3.15+incompatible",
    )
    go_repository(
        name = "com_github_ugorji_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ugorji/go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/68gy2h+1mWMrwZFeD1kQialdSzAb432dtpeJ42ovdo=",
        version = "v1.1.7",
    )
    go_repository(
        name = "com_github_ugorji_go_codec",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ugorji/go/codec",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YPXUKf7fYbp/y8xloBqZOw2qaVggbfwMlI8WM3wZUJ0=",
        version = "v1.2.7",
    )
    go_repository(
        name = "com_github_urfave_cli",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/urfave/cli",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gsqYFH8bb9ekPA12kRo0hfjngWQjkJPlN9R0N78BoUo=",
        version = "v1.22.2",
    )
    go_repository(
        name = "com_github_urfave_cli_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/urfave/cli/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qph92Y649prgesehzOrQjdWyxFOp/QVM+6imKHad91M=",
        version = "v2.3.0",
    )
    go_repository(
        name = "com_github_urfave_negroni",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/urfave/negroni",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kIimOitoypq34K7TG7DUaJ9kq/N4Ofuwi1sjz0KipXc=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_valyala_bytebufferpool",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/bytebufferpool",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GqA5TC/0021Y/b9FG4Oi9Mr3q7XYx6KllzawFIhcdPw=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_valyala_fasthttp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/fasthttp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CRq/00MfruPGFLTQKY8b+8SfdK60TxNztjRMnH0t1Yc=",
        version = "v1.40.0",
    )
    go_repository(
        name = "com_github_valyala_fasttemplate",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/fasttemplate",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lxLXG0uE3Qnshl9QyaK6XJxMXlQZELvChBOCmQD0Loo=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_valyala_tcplisten",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/valyala/tcplisten",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0R4NLDRDZX6JcmhJgXi5E4b8Wg84ihbmUKp/GvSPEzc=",
        version = "v0.0.0-20161114210144-ceec8f93295a",
    )
    go_repository(
        name = "com_github_vektah_gqlparser",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vektah/gqlparser",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ZsyLGn7/7jDNI+y4SEhI4yAxRChlv15pUHMjijT+e68=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_vishvananda_netlink",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vishvananda/netlink",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:cPXZWzzG0NllBLdjWoD1nDfaqu98YMv+OneaKc8sPOA=",
        version = "v1.1.1-0.20201029203352-d40f9887b852",
    )
    go_repository(
        name = "com_github_vishvananda_netns",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vishvananda/netns",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4hwBBUfQCFe3Cym0ZtKyq7L16eZUtYKs+BaHDN6mAns=",
        version = "v0.0.0-20200728191858-db3c7e526aae",
    )
    go_repository(
        name = "com_github_vividcortex_ewma",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/VividCortex/ewma",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MnEK4VOv6n0RSY4vtRe3h11qjxL3+t0B8yOL8iMXdcM=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_github_vividcortex_gohistogram",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/VividCortex/gohistogram",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6+hBz+qvs0JOrrNhhmR7lFxo5sINxBCGXrdtl/UvroE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_vmihailenco_msgpack_v5",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vmihailenco/msgpack/v5",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5gO0H1iULLWGhs2H5tbAHIZTV8/cYafcFOr9znI5mJU=",
        version = "v5.3.5",
    )
    go_repository(
        name = "com_github_vmihailenco_tagparser_v2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/vmihailenco/tagparser/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:y09buUbR+b5aycVFQs/g70pqKVZNBmxwAhO7/IwNM9g=",
        version = "v2.0.0",
    )
    go_repository(
        name = "com_github_wadey_gocovmerge",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/wadey/gocovmerge",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:W0LEBv82YCGEtcmPA3uNZBI33/qF//HAAs3MawDjRa0=",
        version = "v0.0.0-20160331181800-b5bfa59ec0ad",
    )
    go_repository(
        name = "com_github_willf_bitset",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/willf/bitset",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:N7Z7E9UvjW+sGsEl7k/SJrvY2reP1A07MrGuCjIOjRE=",
        version = "v1.1.11",
    )
    go_repository(
        name = "com_github_xdg_go_pbkdf2",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg-go/pbkdf2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Su7DPu48wXMwC3bs7MCNG+z4FhcyEuz5dlvchbq0B0c=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_xdg_go_scram",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg-go/scram",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FHX5I5B4i4hKRVRBCFRxq1iQRej7WO3hhBuJf+UUySY=",
        version = "v1.1.2",
    )
    go_repository(
        name = "com_github_xdg_go_stringprep",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg-go/stringprep",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XLI/Ng3O1Atzq0oBs3TWm+5ZVgkq2aqdlvP9JtoZ6c8=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_github_xdg_scram",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg/scram",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:u40Z8hqBAAQyv+vATcGgV0YCnDjqSL7/q/JyPhhJSPk=",
        version = "v0.0.0-20180814205039-7eeb5667e42c",
    )
    go_repository(
        name = "com_github_xdg_stringprep",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xdg/stringprep",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:n+nNi93yXLkJvKwXNP9d55HC7lGK4H/SRcwB5IaUZLo=",
        version = "v0.0.0-20180714160509-73f8eece6fdc",
    )
    go_repository(
        name = "com_github_xeipuuv_gojsonpointer",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xeipuuv/gojsonpointer",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zGWFAtiMcyryUHoUjUJX0/lt1H2+i2Ka2n+D3DImSNo=",
        version = "v0.0.0-20190905194746-02993c407bfb",
    )
    go_repository(
        name = "com_github_xeipuuv_gojsonreference",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xeipuuv/gojsonreference",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EzJWgHovont7NscjpAxXsDA8S8BMYve8Y5+7cuRE7R0=",
        version = "v0.0.0-20180127040603-bd5ef7bd5415",
    )
    go_repository(
        name = "com_github_xeipuuv_gojsonschema",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xeipuuv/gojsonschema",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LhYJRs+L4fBtjZUfuSZIKGeVu0QRy8e5Xi7D17UxZ74=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_xhit_go_str2duration",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xhit/go-str2duration",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BcV5u025cITWxEQKGWr1URRzrcXtu7uk8+luz3Yuhwc=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_github_xiang90_probing",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xiang90/probing",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:eY9dn8+vbi4tKz5Qo6v2eYzo7kUS51QINcR5jNpbZS8=",
        version = "v0.0.0-20190116061207-43a291ad63a2",
    )
    go_repository(
        name = "com_github_xlab_treeprint",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xlab/treeprint",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:G/1DjNkPpfZCFt9CSh6b5/nY4VimlbHF3Rh4obvtzDk=",
        version = "v1.1.0",
    )
    go_repository(
        name = "com_github_xordataexchange_crypt",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/xordataexchange/crypt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ESFSdwYZvkeru3RtdrYueztKhOBCSAAzS4Gf+k0tEow=",
        version = "v0.0.3-0.20170626215501-b2862e3d0a77",
    )
    go_repository(
        name = "com_github_yalp_jsonpath",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yalp/jsonpath",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6fRhSjgLCkTD3JnJxvaJ4Sj+TYblw757bqYgZaOq5ZY=",
        version = "v0.0.0-20180802001716-5cc68e5049a0",
    )
    go_repository(
        name = "com_github_yosssi_ace",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yosssi/ace",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tUkIP/BLdKqrlrPwcmH0shwEEhTRHoGnc1wFIWmaBUA=",
        version = "v0.0.5",
    )
    go_repository(
        name = "com_github_youmark_pkcs8",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/youmark/pkcs8",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:splanxYIlg+5LfHAM6xpdFEAYOk8iySO56hMFq6uLyA=",
        version = "v0.0.0-20181117223130-1be2e3e5546d",
    )
    go_repository(
        name = "com_github_yudai_gojsondiff",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yudai/gojsondiff",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:27cbfqXLVEJ1o8I6v3y9lg8Ydm53EKqHXAOMxEGlCOA=",
        version = "v1.0.0",
    )
    go_repository(
        name = "com_github_yudai_golcs",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yudai/golcs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BHyfKlQyqbsFN5p3IfnEUduWvb9is428/nNb5L3U01M=",
        version = "v0.0.0-20170316035057-ecda9a501e82",
    )
    go_repository(
        name = "com_github_yudai_pp",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yudai/pp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Q4//iY4pNF6yPLZIigmvcl7k/bPgrcTPIFIcmawg5bI=",
        version = "v2.0.1+incompatible",
    )
    go_repository(
        name = "com_github_yuin_goldmark",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yuin/goldmark",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fVcFKWvrslecOb/tg+Cc05dkeYx540o0FuFt3nUVDoE=",
        version = "v1.4.13",
    )
    go_repository(
        name = "com_github_yusufpapurcu_wmi",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yusufpapurcu/wmi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KBNDSne4vP5mbSWnJbO+51IMOXJB67QiYCSBrubbPRg=",
        version = "v1.2.2",
    )
    go_repository(
        name = "com_github_yvasiyarov_go_metrics",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yvasiyarov/go-metrics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+lm10QQTNSBd8DVTNGHx7o/IKu9HYDvLMffDhbyLccI=",
        version = "v0.0.0-20140926110328-57bccd1ccd43",
    )
    go_repository(
        name = "com_github_yvasiyarov_gorelic",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yvasiyarov/gorelic",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hlE8//ciYMztlGpl/VA+Zm1AcTPHYkHJPbHqE6WJUXE=",
        version = "v0.0.0-20141212073537-a9bba5b9ab50",
    )
    go_repository(
        name = "com_github_yvasiyarov_newrelic_platform_go",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/yvasiyarov/newrelic_platform_go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ERexzlUfuTvpE74urLSbIQW0Z/6hF9t8U4NsJLaioAY=",
        version = "v0.0.0-20140908184405-b21fdbd4370f",
    )
    go_repository(
        name = "com_github_z_division_go_zookeeper",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/z-division/go-zookeeper",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Itr7GbuXoM1PK/eCeNNia4Qd3ib9IgX9g9SpXgo8BwQ=",
        version = "v0.0.0-20190128072838-6d7457066b9b",
    )
    go_repository(
        name = "com_github_zabawaba99_go_gitignore",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/zabawaba99/go-gitignore",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vpcCVk+pSR/6zcurmlGFD3jC5I/7RMl+GwGAPLxvX18=",
        version = "v0.0.0-20200117185801-39e6bddfb292",
    )
    go_repository(
        name = "com_github_zenazn_goji",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/zenazn/goji",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RSQQAbXGArQ0dIDEq+PI6WqN6if+5KHu6x2Cx/GXLTQ=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_github_ziutek_mymysql",
        build_file_proto_mode = "disable_global",
        importpath = "github.com/ziutek/mymysql",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GB0qdRGsTwQSBVYuVShFBKaXSnSnYYC2d9knnE1LHFs=",
        version = "v1.5.4",
    )
    go_repository(
        name = "com_google_cloud_go",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uJSeirPke5UNZHIb4SxfZklVSiWWVqW4oXlETwZziwM=",
        version = "v0.112.1",
    )
    go_repository(
        name = "com_google_cloud_go_accessapproval",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/accessapproval",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uzmAMSgYcnlHa9X9YSQZ4Q1wlfl4NNkZyQgho1Z6p04=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_accesscontextmanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/accesscontextmanager",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2GLNaNu9KRJhJBFTIVRoPwk6xE5mUDgD47abBq4Zp/I=",
        version = "v1.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_aiplatform",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/aiplatform",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0cSrii1ZeLr16MbBoocyy5KVnrSdiQ3KN/vtrTe7RqE=",
        version = "v1.60.0",
    )
    go_repository(
        name = "com_google_cloud_go_analytics",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/analytics",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Q+y94XH84jM8SK8O7qiY/PJRexb6n7dRbQ6PiUa4YGM=",
        version = "v0.23.0",
    )
    go_repository(
        name = "com_google_cloud_go_apigateway",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/apigateway",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sPXnpk+6TneKIrjCjcpX5YGsAKy3PTdpIchoj8/74OE=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_apigeeconnect",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/apigeeconnect",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CrfIKv9Go3fh/QfQgisU3MeP90Ww7l/sVGmr3TpECo8=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_apigeeregistry",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/apigeeregistry",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:C+QU2K+DzDjk4g074ouwHQGkoff1h5OMQp6sblCVreQ=",
        version = "v0.8.3",
    )
    go_repository(
        name = "com_google_cloud_go_appengine",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/appengine",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:l2SviT44zWQiOv8bPoMBzW0vOcMO22iO0s+nVtVhdts=",
        version = "v1.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_area120",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/area120",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vTs08KPLN/iMzTbxpu5ciL06KcsrVPMjz4IwcQyZ4uY=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_artifactregistry",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/artifactregistry",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:W9sVlyb1VRcUf83w7aM3yMsnp4HS4PoyGqYQNG0O5lI=",
        version = "v1.14.7",
    )
    go_repository(
        name = "com_google_cloud_go_asset",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/asset",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xgFnBP3luSbUcC9RWJvb3Zkt+y/wW6PKwPHr3ssnIP8=",
        version = "v1.17.2",
    )
    go_repository(
        name = "com_google_cloud_go_assuredworkloads",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/assuredworkloads",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gCrN3IyvqY3cP0wh2h43d99CgH3G+WYs9CeuFVKChR8=",
        version = "v1.11.5",
    )
    go_repository(
        name = "com_google_cloud_go_auth",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/auth",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uiha352VrCDMXg+yoBtaD0tUF4Kv9vrtrWPYXwutnDE=",
        version = "v0.7.2",
    )
    go_repository(
        name = "com_google_cloud_go_auth_oauth2adapt",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/auth/oauth2adapt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+TTV8aXpjeChS9M+aTtN/TjdQnzJvmzKFt//oWu7HX4=",
        version = "v0.2.2",
    )
    go_repository(
        name = "com_google_cloud_go_automl",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/automl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ijiJy9sYWh75WrqImXsfWc1e3HR3iO+ef9fvW03Ig/4=",
        version = "v1.13.5",
    )
    go_repository(
        name = "com_google_cloud_go_baremetalsolution",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/baremetalsolution",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LFydisRmS7hQk9P/YhekwuZGqb45TW4QavcrMToWo5A=",
        version = "v1.2.4",
    )
    go_repository(
        name = "com_google_cloud_go_batch",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/batch",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2HK4JerwVaIcCh/lJiHwh6+uswPthiMMWhiSWLELayk=",
        version = "v1.8.0",
    )
    go_repository(
        name = "com_google_cloud_go_beyondcorp",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/beyondcorp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qs0J0O9Ol2h1yA0AU+r7l3hOCPzs2MjE1d6d/kaHIKo=",
        version = "v1.0.4",
    )
    go_repository(
        name = "com_google_cloud_go_bigquery",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/bigquery",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CpT+/njKuKT3CEmswm6IbhNu9u35zt5dO4yPDLW+nG4=",
        version = "v1.59.1",
    )
    go_repository(
        name = "com_google_cloud_go_bigtable",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/bigtable",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:PAplkJLXheOLlK5PPyy4/HXtPzHn+1/LaYDWIeGxnio=",
        version = "v1.3.0",
    )
    go_repository(
        name = "com_google_cloud_go_billing",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/billing",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oWUEQvuC4JvtnqLZ35zgzdbuHt4Itbftvzbe6aEyFdE=",
        version = "v1.18.2",
    )
    go_repository(
        name = "com_google_cloud_go_binaryauthorization",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/binaryauthorization",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1jcyh2uIUwSZkJ/JmL8kd5SUkL/Krbv8zmYLEbAz6kY=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_google_cloud_go_certificatemanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/certificatemanager",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UMBr/twXvH3jcT5J5/YjRxf2tvwTYIfrpemTebe0txc=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_channel",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/channel",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/omiBnyFjm4S1ETHoOmJbL7LH7Ljcei4rYG6Sj3hc80=",
        version = "v1.17.5",
    )
    go_repository(
        name = "com_google_cloud_go_cloudbuild",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/cloudbuild",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ZB6oOmJo+MTov9n629fiCrO9YZPOg25FZvQ7gIHu5ng=",
        version = "v1.15.1",
    )
    go_repository(
        name = "com_google_cloud_go_clouddms",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/clouddms",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Sr0Zo5EAcPQiCBgHWICg3VGkcdS/LLP1d9SR7qQBM/s=",
        version = "v1.7.4",
    )
    go_repository(
        name = "com_google_cloud_go_cloudtasks",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/cloudtasks",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EUt1hIZ9bLv8Iz9yWaCrqgMnIU+Tdh0yXM1MMVGhjfE=",
        version = "v1.12.6",
    )
    go_repository(
        name = "com_google_cloud_go_compute",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/compute",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:phWcR2eWzRJaL/kOiJwfFsPs4BaKq1j6vnpZrc1YlVg=",
        version = "v1.24.0",
    )
    go_repository(
        name = "com_google_cloud_go_compute_metadata",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/compute/metadata",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pDUj4QMoPejqq20dK0Pg2N4yG9zIkYGdBtwLoEkH9Zs=",
        version = "v0.9.0",
    )
    go_repository(
        name = "com_google_cloud_go_contactcenterinsights",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/contactcenterinsights",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:6Vs/YnDG5STGjlWMEjN/xtmft7MrOTOnOZYUZtGTx0w=",
        version = "v1.13.0",
    )
    go_repository(
        name = "com_google_cloud_go_container",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/container",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:MAaNH7VRNPWEhvqOypq2j+7ONJKrKzon4v9nS3nLZe0=",
        version = "v1.31.0",
    )
    go_repository(
        name = "com_google_cloud_go_containeranalysis",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/containeranalysis",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:doJ0M1ljS4hS0D2UbHywlHGwB7sQLNrt9vFk9Zyi7vY=",
        version = "v0.11.4",
    )
    go_repository(
        name = "com_google_cloud_go_datacatalog",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datacatalog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:A0vKYCQdxQuV4Pi0LL9p39Vwvg4jH5yYveMv50gU5Tw=",
        version = "v1.19.3",
    )
    go_repository(
        name = "com_google_cloud_go_dataflow",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataflow",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RYHtcPhmE664+F0Je46p+NvFbG8z//KCXp+uEqB4jZU=",
        version = "v0.9.5",
    )
    go_repository(
        name = "com_google_cloud_go_dataform",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataform",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5e4eqGrd0iDTCg4Q+VlAao5j2naKAA7xRurNtwmUknU=",
        version = "v0.9.2",
    )
    go_repository(
        name = "com_google_cloud_go_datafusion",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datafusion",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HQ/BUOP8OIGJxuztpYvNvlb+/U+/Bfs9SO8tQbh61fk=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_datalabeling",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datalabeling",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GpIFRdm0qIZNsxqURFJwHt0ZBJZ0nF/mUVEigR7PH/8=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_dataplex",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataplex",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fxIfdU8fxzR3clhOoNI7XFppvAmndxDu1AMH+qX9WKQ=",
        version = "v1.14.2",
    )
    go_repository(
        name = "com_google_cloud_go_dataproc_v2",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataproc/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/u81Fd+BvCLp+xjctI1DiWVJn6cn9/s3Akc8xPH02yk=",
        version = "v2.4.0",
    )
    go_repository(
        name = "com_google_cloud_go_dataqna",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dataqna",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9ybXs3nr9BzxSGC04SsvtuXaHY0qmJSLIpIAbZo9GqQ=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_datastore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datastore",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0P9WcsQeTWjuD1H14JIY7XQscIPQ4Laje8ti96IC5vg=",
        version = "v1.15.0",
    )
    go_repository(
        name = "com_google_cloud_go_datastream",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/datastream",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:o1QDKMo/hk0FN7vhoUQURREuA0rgKmnYapB+1M+7Qz4=",
        version = "v1.10.4",
    )
    go_repository(
        name = "com_google_cloud_go_deploy",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/deploy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:m27Ojwj03gvpJqCbodLYiVmE9x4/LrHGGMjzc0LBfM4=",
        version = "v1.17.1",
    )
    go_repository(
        name = "com_google_cloud_go_dialogflow",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dialogflow",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KqG0oxGE71qo0lRVyAoeBozefCvsMfcDzDjoLYSY0F4=",
        version = "v1.49.0",
    )
    go_repository(
        name = "com_google_cloud_go_dlp",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/dlp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lTipOuJaSjlYnnotPMbEhKURLC6GzCMDDzVbJAEbmYM=",
        version = "v1.11.2",
    )
    go_repository(
        name = "com_google_cloud_go_documentai",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/documentai",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lI62GMEEPO6vXJI9hj+G9WjOvnR0hEjvjokrnex4cxA=",
        version = "v1.25.0",
    )
    go_repository(
        name = "com_google_cloud_go_domains",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/domains",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Mml/R6s3vQQvFPpi/9oX3O5dRirgjyJ8cksK8N19Y7g=",
        version = "v0.9.5",
    )
    go_repository(
        name = "com_google_cloud_go_edgecontainer",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/edgecontainer",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:tBY32km78ScpK2aOP84JoW/+wtpx5WluyPUSEE3270U=",
        version = "v1.1.5",
    )
    go_repository(
        name = "com_google_cloud_go_errorreporting",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/errorreporting",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kj1XEWMu8P0qlLhm3FwcaFsUvXChV/OraZwA70trRR0=",
        version = "v0.3.0",
    )
    go_repository(
        name = "com_google_cloud_go_essentialcontacts",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/essentialcontacts",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:13eHn5qBnsawxI7mIrv4jRIEmQ1xg0Ztqw5ZGqtUNfA=",
        version = "v1.6.6",
    )
    go_repository(
        name = "com_google_cloud_go_eventarc",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/eventarc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ORkd6/UV5FIdA8KZQDLNZYKS7BBOrj0p01DXPmT4tE4=",
        version = "v1.13.4",
    )
    go_repository(
        name = "com_google_cloud_go_filestore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/filestore",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:X5G4y/vrUo1B8Nsz93qSWTMAcM8LXbGUldq33OdcdCw=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_google_cloud_go_firestore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/firestore",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8aLcKnMPoldYU3YHgu4t2exrKhLQkqaXAGqT0ljrFVw=",
        version = "v1.14.0",
    )
    go_repository(
        name = "com_google_cloud_go_functions",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/functions",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IWVylmK5F6hJ3R5zaRW7jI5PrWhCvtBVU4axQLmXSo4=",
        version = "v1.16.0",
    )
    go_repository(
        name = "com_google_cloud_go_gkebackup",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkebackup",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iuE8KNtTsPOc79qeWoNS8zOWoXPD9SAdOmwgxtlCmh8=",
        version = "v1.3.5",
    )
    go_repository(
        name = "com_google_cloud_go_gkeconnect",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkeconnect",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:17d+ZSSXKqG/RwZCq3oFMIWLPI8Zw3b8+a9/BEVlwH0=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_gkehub",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkehub",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RboLNFzf9wEMSo7DrKVBlf+YhK/A/jrLN454L5Tz99Q=",
        version = "v0.14.5",
    )
    go_repository(
        name = "com_google_cloud_go_gkemulticloud",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gkemulticloud",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rsSZAGLhyjyE/bE2ToT5fqo1qSW7S+Ubsc9jFOcbhSI=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_google_cloud_go_gsuiteaddons",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/gsuiteaddons",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CZEbaBwmbYdhFw21Fwbo+C35HMe36fTE0FBSR4KSfWg=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_iam",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/iam",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:z4VHOhwKLF/+UYXAJDFwGtNF0b6gjsW1Pk9Ml0U/IoM=",
        version = "v1.1.7",
    )
    go_repository(
        name = "com_google_cloud_go_iap",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/iap",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:94zirc2r4t6KzhAMW0R6Dme005eTP6yf7g6vN4IhRrA=",
        version = "v1.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_ids",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/ids",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xd4U7pgl3GHV+MABnv1BF4/Vy/zBF7CYC8XngkOLzag=",
        version = "v1.4.5",
    )
    go_repository(
        name = "com_google_cloud_go_iot",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/iot",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:munTeBlbqI33iuTYgXy7S8lW2TCgi5l1hA4roSIY+EE=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_kms",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/kms",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7caV9K3yIxvlQPAcaFffhlT7d1qpxjB1wHBtjWa13SM=",
        version = "v1.15.7",
    )
    go_repository(
        name = "com_google_cloud_go_language",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/language",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iaJZg6K4j/2PvZZVcjeO/btcWWIllVRBhuTFjGO4LXs=",
        version = "v1.12.3",
    )
    go_repository(
        name = "com_google_cloud_go_lifesciences",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/lifesciences",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:gXvN70m2p+4zgJFzaz6gMKaxTuF9WJ0USYoMLWAOm8g=",
        version = "v0.9.5",
    )
    go_repository(
        name = "com_google_cloud_go_logging",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/logging",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iEIOXFO9EmSiTjDmfpbRjOxECO7R8C7b8IXUGOj7xZw=",
        version = "v1.9.0",
    )
    go_repository(
        name = "com_google_cloud_go_longrunning",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/longrunning",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xAe8+0YaWoCKr9t1+aWe+OeQgN/iJK1fEgZSXmjuEaE=",
        version = "v0.5.6",
    )
    go_repository(
        name = "com_google_cloud_go_managedidentities",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/managedidentities",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+bpih1piZVLxla/XBqeSUzJBp8gv9plGHIMAI7DLpDM=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_maps",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/maps",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EVCZAiDvog9So46460BGbCasPhi613exoaQbpilMVlk=",
        version = "v1.6.4",
    )
    go_repository(
        name = "com_google_cloud_go_mediatranslation",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/mediatranslation",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:c76KdIXljQHSCb/Cy47S8H4s05A4zbK3pAFGzwcczZo=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_memcache",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/memcache",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yeDv5qxRedFosvpMSEswrqUsJM5OdWvssPHFliNFTc4=",
        version = "v1.10.5",
    )
    go_repository(
        name = "com_google_cloud_go_metastore",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/metastore",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dR7vqWXlK6IYR8Wbu9mdFfwlVjodIBhd1JRrpZftTEg=",
        version = "v1.13.4",
    )
    go_repository(
        name = "com_google_cloud_go_monitoring",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/monitoring",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:NfkDLQDG2UR3WYZVQE8kwSbUIEyIqJUPl+aOQdFH1T4=",
        version = "v1.18.0",
    )
    go_repository(
        name = "com_google_cloud_go_networkconnectivity",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/networkconnectivity",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GBfXFhLyPspnaBE3nI/BRjdhW8vcbpT9QjE/4kDCDdc=",
        version = "v1.14.4",
    )
    go_repository(
        name = "com_google_cloud_go_networkmanagement",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/networkmanagement",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:aLV5GcosBNmd6M8+a0ekB0XlLRexv4fvnJJrYnqeBcg=",
        version = "v1.9.4",
    )
    go_repository(
        name = "com_google_cloud_go_networksecurity",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/networksecurity",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:+caSxBTj0E8OYVh/5wElFdjEMO1S/rZtE1152Cepchc=",
        version = "v0.9.5",
    )
    go_repository(
        name = "com_google_cloud_go_notebooks",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/notebooks",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:FH48boYmrWVQ6k0Mx/WrnNafXncT5iSYxA8CNyWTgy0=",
        version = "v1.11.3",
    )
    go_repository(
        name = "com_google_cloud_go_optimization",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/optimization",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:63NZaWyN+5rZEKHPX4ACpw3BjgyeuY8+rCehiCMaGPY=",
        version = "v1.6.3",
    )
    go_repository(
        name = "com_google_cloud_go_orchestration",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/orchestration",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YHgWMlrPttIVGItgGfuvO2KM7x+y9ivN/Yk92pMm1a4=",
        version = "v1.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_orgpolicy",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/orgpolicy",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2JbXigqBJVp8Dx5dONUttFqewu4fP0p3pgOdIZAhpYU=",
        version = "v1.12.1",
    )
    go_repository(
        name = "com_google_cloud_go_osconfig",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/osconfig",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Mo5jGAxOMKH/PmDY7fgY19yFcVbvwREb5D5zMPQjFfo=",
        version = "v1.12.5",
    )
    go_repository(
        name = "com_google_cloud_go_oslogin",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/oslogin",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1K4nOT5VEZNt7XkhaTXupBYos5HjzvJMfhvyD2wWdFs=",
        version = "v1.13.1",
    )
    go_repository(
        name = "com_google_cloud_go_phishingprotection",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/phishingprotection",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DH3WFLzEoJdW/6xgsmoDqOwT1xddFi7gKu0QGZQhpGU=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_policytroubleshooter",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/policytroubleshooter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:c0WOzC6hz964QWNBkyKfna8A2jOIx1zzZa43Gx/P09o=",
        version = "v1.10.3",
    )
    go_repository(
        name = "com_google_cloud_go_privatecatalog",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/privatecatalog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UZ0assTnATXSggoxUIh61RjTQ4P9zCMk/kEMbn0nMYA=",
        version = "v0.9.5",
    )
    go_repository(
        name = "com_google_cloud_go_pubsub",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/pubsub",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dfEPuGCHGbWUhaMCTHUFjfroILEkx55iUmKBZTP5f+Y=",
        version = "v1.36.1",
    )
    go_repository(
        name = "com_google_cloud_go_pubsublite",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/pubsublite",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pX+idpWMIH30/K7c0epN6V703xpIcMXWRjKJsz0tYGY=",
        version = "v1.8.1",
    )
    go_repository(
        name = "com_google_cloud_go_recaptchaenterprise_v2",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/recaptchaenterprise/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:U3Wfq12X9cVMuTpsWDSURnXF0Z9hSPTHj+xsnXDRLsw=",
        version = "v2.9.2",
    )
    go_repository(
        name = "com_google_cloud_go_recommendationengine",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/recommendationengine",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ineqLswaCSBY0csYv5/wuXJMBlxATK6Xc5jJkpiTEdM=",
        version = "v0.8.5",
    )
    go_repository(
        name = "com_google_cloud_go_recommender",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/recommender",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LVLYS3r3u0MSCxQSDUtLSkporEGi9OAE6hGvayrZNPs=",
        version = "v1.12.1",
    )
    go_repository(
        name = "com_google_cloud_go_redis",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/redis",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:QF0maEdVv0Fj/2roU8sX3NpiDBzP9ICYTO+5F32gQNo=",
        version = "v1.14.2",
    )
    go_repository(
        name = "com_google_cloud_go_resourcemanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/resourcemanager",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AZWr1vWVDKGwfLsVhcN+vcwOz3xqqYxtmMa0aABCMms=",
        version = "v1.9.5",
    )
    go_repository(
        name = "com_google_cloud_go_resourcesettings",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/resourcesettings",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BTr5MVykJwClASci/7Og4Qfx70aQ4n3epsNLj94ZYgw=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_retail",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/retail",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Fn1GuAua1c6crCGqfJ1qMxG1Xh10Tg/x5EUODEHMqkw=",
        version = "v1.16.0",
    )
    go_repository(
        name = "com_google_cloud_go_run",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/run",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:m9WDA7DzTpczhZggwYlZcBWgCRb+kgSIisWn1sbw2rQ=",
        version = "v1.3.4",
    )
    go_repository(
        name = "com_google_cloud_go_scheduler",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/scheduler",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5U8iXLoQ03qOB+ZXlAecU7fiE33+u3QiM9nh4cd0eTE=",
        version = "v1.10.6",
    )
    go_repository(
        name = "com_google_cloud_go_secretmanager",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/secretmanager",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:82fpF5vBBvu9XW4qj0FU2C6qVMtj1RM/XHwKXUEAfYY=",
        version = "v1.11.5",
    )
    go_repository(
        name = "com_google_cloud_go_security",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/security",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wTKJQ10j8EYgvE8Y+KhovxDRVDk2iv/OsxZ6GrLP3kE=",
        version = "v1.15.5",
    )
    go_repository(
        name = "com_google_cloud_go_securitycenter",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/securitycenter",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/5jjkZ+uGe8hZ7pvd7pO30VW/a+pT2MrrdgOqjyucKQ=",
        version = "v1.24.4",
    )
    go_repository(
        name = "com_google_cloud_go_servicedirectory",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/servicedirectory",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:da7HFI1229kyzIyuVEzHXip0cw0d+E0s8mjQby0WN+k=",
        version = "v1.11.4",
    )
    go_repository(
        name = "com_google_cloud_go_shell",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/shell",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:3Fq2hzO0ZSyaqBboJrFkwwf/qMufDtqwwA6ep8EZxEI=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_spanner",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/spanner",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:o/Cv7/zZ1WgRXVCd5g3Nc23ZI39p/1pWFqFwvg6Wcu8=",
        version = "v1.56.0",
    )
    go_repository(
        name = "com_google_cloud_go_speech",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/speech",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nuFc+Kj5B8de75nN4FdPyUbI2SiBoHZG6BLurXL56Q0=",
        version = "v1.21.1",
    )
    go_repository(
        name = "com_google_cloud_go_storage",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/storage",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VEpDQV5CJxFmJ6ueWNsKxcr1QAYOXEgxDa+sBbJahPw=",
        version = "v1.40.0",
    )
    go_repository(
        name = "com_google_cloud_go_storagetransfer",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/storagetransfer",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dy4fL3wO0VABvzM05ycMUPFHxTPbJz9Em8ikAJVqSbI=",
        version = "v1.10.4",
    )
    go_repository(
        name = "com_google_cloud_go_talent",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/talent",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JssV0CE3FNujuSWn7SkosOzg7qrMxVnt6txOfGcMSa4=",
        version = "v1.6.6",
    )
    go_repository(
        name = "com_google_cloud_go_texttospeech",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/texttospeech",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dxY2Q5mHCbrGa3oPR2O3PCicdnvKa1JmwGQK36EFLOw=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_tpu",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/tpu",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:C8YyYda8WtNdBoCgFwwBzZd+S6+EScHOxM/z1h0NNp8=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_trace",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/trace",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0pr4lIKJ5XZFYD9GtxXEWr0KkVeigc3wlGpZco0X1oA=",
        version = "v1.10.5",
    )
    go_repository(
        name = "com_google_cloud_go_translate",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/translate",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:upovZ0wRMdzZvXnu+RPam41B0mRJ+coRXFP2cYFJ7ew=",
        version = "v1.10.1",
    )
    go_repository(
        name = "com_google_cloud_go_video",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/video",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TXwotxkShP1OqgKsbd+b8N5hrIHavSyLGvYnLGCZ7xc=",
        version = "v1.20.4",
    )
    go_repository(
        name = "com_google_cloud_go_videointelligence",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/videointelligence",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mYaWH8uhUCXLJCN3gdXswKzRa2+lK0zN6/KsIubm6pE=",
        version = "v1.11.5",
    )
    go_repository(
        name = "com_google_cloud_go_vision_v2",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vision/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:W52z1b6LdGI66MVhE70g/NFty9zCYYcjdKuycqmlhtg=",
        version = "v2.8.0",
    )
    go_repository(
        name = "com_google_cloud_go_vmmigration",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vmmigration",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:5v9RT2vWyuw3pK2ox0HQpkoftO7Q7/8591dTxxQc79g=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_vmwareengine",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vmwareengine",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:EGdDi9QbqThfZq3ILcDK5g+m9jTevc34AY5tACx5v7k=",
        version = "v1.1.1",
    )
    go_repository(
        name = "com_google_cloud_go_vpcaccess",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/vpcaccess",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XyL6hTLtEM/eE4F1GEge8xUN9ZCkiVWn44K/YA7z1rQ=",
        version = "v1.7.5",
    )
    go_repository(
        name = "com_google_cloud_go_webrisk",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/webrisk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:251MvGuC8wisNN7+jqu9DDDZAi38KiMXxOpA/EWy4dE=",
        version = "v1.9.5",
    )
    go_repository(
        name = "com_google_cloud_go_websecurityscanner",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/websecurityscanner",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YqWZrZYabG88TZt7364XWRJGhxmxhony2ZUyZEYMF2k=",
        version = "v1.6.5",
    )
    go_repository(
        name = "com_google_cloud_go_workflows",
        build_file_proto_mode = "disable_global",
        importpath = "cloud.google.com/go/workflows",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uHNmUiatTbPQ4H1pabwfzpfEYD4BBnqDHqMm2IesOh4=",
        version = "v1.12.4",
    )
    go_repository(
        name = "com_lukechampine_uint128",
        build_file_proto_mode = "disable_global",
        importpath = "lukechampine.com/uint128",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mBi/5l91vocEN8otkC5bDLhi2KdCticRiwbdB0O+rjI=",
        version = "v1.2.0",
    )
    go_repository(
        name = "com_shuralyov_dmitri_gpu_mtl",
        build_file_proto_mode = "disable_global",
        importpath = "dmitri.shuralyov.com/gpu/mtl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VpgP7xuJadIUuKccphEpTJnWhS2jkQyMt6Y7pJCD7fY=",
        version = "v0.0.0-20190408044501-666a987793e9",
    )
    go_repository(
        name = "com_sourcegraph_sourcegraph_appdash",
        build_file_proto_mode = "disable_global",
        importpath = "sourcegraph.com/sourcegraph/appdash",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ucqkfpjg9WzSUubAO62csmucvxl4/JeW3F4I4909XkM=",
        version = "v0.0.0-20190731080439-ebfcffb1b5c0",
    )
    go_repository(
        name = "dev_cel_expr",
        build_file_proto_mode = "disable_global",
        importpath = "cel.dev/expr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1KrZg61W6TWSxuNZ37Xy49ps13NUovb66QLprthtwi4=",
        version = "v0.25.1",
    )
    go_repository(
        name = "ht_sr_git_~sbinet_gg",
        build_file_proto_mode = "disable_global",
        importpath = "git.sr.ht/~sbinet/gg",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:LNhjNn8DerC8f9DHLz6lS0YYul/b602DUxDgGkd/Aik=",
        version = "v0.3.1",
    )
    go_repository(
        name = "in_gopkg_airbrake_gobrake_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/airbrake/gobrake.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7z2uVWwn7oVeeugY1DtlPAy5H+KYgB1KeKTnqjNatLo=",
        version = "v2.0.9",
    )
    go_repository(
        name = "in_gopkg_alecthomas_kingpin_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/alecthomas/kingpin.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jMFz6MfLP0/4fUyZle81rXUoxOBFi19VUFKVDOQfozc=",
        version = "v2.2.6",
    )
    go_repository(
        name = "in_gopkg_asn1_ber_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/asn1-ber.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TxyelI5cVkbREznMhfzycHdkp5cLA7DpE+GKjSslYhM=",
        version = "v1.0.0-20181015200546-f715ec2f112d",
    )
    go_repository(
        name = "in_gopkg_check_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/check.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Hei/4ADfdWqJk1ZMxUNpqntNwaWcugrBjAiHlqqRiVk=",
        version = "v1.0.0-20201130134442-10cb98267c6c",
    )
    go_repository(
        name = "in_gopkg_cheggaaa_pb_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/cheggaaa/pb.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Ev7yu1/f6+d+b3pi5vPdRPc6nNtP1umSfcWiEfRqv6I=",
        version = "v1.0.25",
    )
    go_repository(
        name = "in_gopkg_datadog_dd_trace_go_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/DataDog/dd-trace-go.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:j9vAp9Re9bbtA/QFehkJpNba/6W2IbJtNuXZophCa54=",
        version = "v1.17.0",
    )
    go_repository(
        name = "in_gopkg_errgo_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/errgo.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0vLT13EuvQ0hNvakwLuFZ/jYrLp5F3kcWHXdRggjCE8=",
        version = "v2.1.0",
    )
    go_repository(
        name = "in_gopkg_fsnotify_fsnotify_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/fsnotify/fsnotify.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XNNYLJHt73EyYiCZi6+xjupS9CpvmiDgjPTAjrBlQbo=",
        version = "v1.4.7",
    )
    go_repository(
        name = "in_gopkg_fsnotify_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/fsnotify.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xOHLXZwVvI9hhs+cLKq5+I5onOuwQLhQwiu63xxlHs4=",
        version = "v1.4.7",
    )
    go_repository(
        name = "in_gopkg_gcfg_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/gcfg.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:m8OOJ4ccYHnx2f4gQwpno8nAX5OGOh7RLaaz0pj3Ogs=",
        version = "v1.2.3",
    )
    go_repository(
        name = "in_gopkg_gemnasium_logrus_airbrake_hook_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/gemnasium/logrus-airbrake-hook.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:OAj3g0cR6Dx/R07QgQe8wkA9RNjB2u4i700xBkIT4e0=",
        version = "v2.1.2",
    )
    go_repository(
        name = "in_gopkg_go_playground_assert_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/go-playground/assert.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xoYuJVE7KT85PYWrN730RguIQO0ePzVRfFMXadIrXTM=",
        version = "v1.2.1",
    )
    go_repository(
        name = "in_gopkg_go_playground_validator_v8",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/go-playground/validator.v8",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:lFB4DoMU6B626w8ny76MV7VX6W2VHct2GVOI3xgiMrQ=",
        version = "v8.18.2",
    )
    go_repository(
        name = "in_gopkg_go_playground_validator_v9",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/go-playground/validator.v9",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:SvGtYmN60a5CVKTOzMSyfzWDeZRxRuGvRQyEAKbw1xc=",
        version = "v9.29.1",
    )
    go_repository(
        name = "in_gopkg_inconshreveable_log15_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/inconshreveable/log15.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:RlWgLqCMMIYYEVcAR5MDsuHlVkaIPDAF+5Dehzg8L5A=",
        version = "v2.0.0-20180818164646-67afb5ed74ec",
    )
    go_repository(
        name = "in_gopkg_inf_v0",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/inf.v0",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:73M5CoZyi3ZLMOyDlQh031Cx6N9NDJ2Vvfl76EDAgDc=",
        version = "v0.9.1",
    )
    go_repository(
        name = "in_gopkg_ini_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/ini.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Dgnx+6+nfE+IfzjUEISNeydPJh9AXNNsWbGP9KzCsOA=",
        version = "v1.67.0",
    )
    go_repository(
        name = "in_gopkg_ldap_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/ldap.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1rO3ojzsHUk+gq4ZYhC4Pg+EzWaaKIV8+DJwExS5/QQ=",
        version = "v2.5.0",
    )
    go_repository(
        name = "in_gopkg_mgo_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/mgo.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xcEWjVhvbDy+nHP67nPDDpbYrY+ILlfndk4bRioVHaU=",
        version = "v2.0.0-20180705113604-9856a29383ce",
    )
    go_repository(
        name = "in_gopkg_natefinch_lumberjack_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/natefinch/lumberjack.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1Lc07Kr7qY4U2YPouBjpCLxpiyxIVoxqXgkXLknAOE8=",
        version = "v2.0.0",
    )
    go_repository(
        name = "in_gopkg_resty_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/resty.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CuXP0Pjfw9rOuY6EP+UvtNvt5DSqHpIxILZKT/quCZI=",
        version = "v1.12.0",
    )
    go_repository(
        name = "in_gopkg_square_go_jose_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/square/go-jose.v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7odma5RETjNHWJnR32wx8t+Io4djHE1PqxCFx3iiZ2w=",
        version = "v2.5.1",
    )
    go_repository(
        name = "in_gopkg_src_d_go_billy_v4",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/src-d/go-billy.v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KtlZ4c1OWbIs4jCv5ZXrTqG8EQocr0g/d4DjNg70aek=",
        version = "v4.3.0",
    )
    go_repository(
        name = "in_gopkg_tomb_v1",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/tomb.v1",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uRGJdciOHaEIrze2W8Q3AKkepLTh2hOroT7a+7czfdQ=",
        version = "v1.0.0-20141024135613-dd632973f1e7",
    )
    go_repository(
        name = "in_gopkg_warnings_v0",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/warnings.v0",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wFXVbFY8DY5/xOe1ECiWdKCzZlxgshcYVNkBHstARME=",
        version = "v0.1.2",
    )
    go_repository(
        name = "in_gopkg_yaml_v2",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/yaml.v2",
        replace = "github.com/cockroachdb/yaml",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:vVVz+IAeHhYPxGW9EC8j6HR7uZl/1wSP0Wijaxs4frw=",
        version = "v0.0.0-20210825132133-2d6955c8edbc",
    )
    go_repository(
        name = "in_gopkg_yaml_v3",
        build_file_proto_mode = "disable_global",
        importpath = "gopkg.in/yaml.v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
        version = "v3.0.1",
    )
    go_repository(
        name = "io_etcd_go_bbolt",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/bbolt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XAzx9gjCb0Rxj7EoqcClPD1d5ZBxZJk0jbuoPHenBt0=",
        version = "v1.3.5",
    )
    go_repository(
        name = "io_etcd_go_etcd",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1JFLBqwIgdyHN1ZtgjTBwO+blA6gVOmZurpiMEsETKo=",
        version = "v0.5.0-alpha.5.0.20200910180754-dd1b699fc489",
    )
    go_repository(
        name = "io_etcd_go_etcd_api_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/api/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:GsV3S+OfZEOCNXdtNkBSR7kgLobAa/SO6tCxRa0GAYw=",
        version = "v3.5.0",
    )
    go_repository(
        name = "io_etcd_go_etcd_client_pkg_v3",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/client/pkg/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2aQv6F436YnN7I4VbI8PPYrBhu+SmrTaADcf8Mi/6PU=",
        version = "v3.5.0",
    )
    go_repository(
        name = "io_etcd_go_etcd_client_v2",
        build_file_proto_mode = "disable_global",
        importpath = "go.etcd.io/etcd/client/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ftQ0nOOHMcbMS3KIaDQ0g5Qcd6bhaBrQT6b89DfwLTs=",
        version = "v2.305.0",
    )
    go_repository(
        name = "io_etcd_go_raft_v3",
        build_directives = [
            "gazelle:resolve proto proto gogoproto/gogo.proto @com_github_gogo_protobuf//gogoproto:gogo_proto",
            "gazelle:resolve proto go gogoproto/gogo.proto @com_github_gogo_protobuf//gogoproto",
            "gazelle:go_proto_compilers @com_github_cockroachdb_cockroach//pkg/cmd/protoc-gen-gogoroach:protoc-gen-gogoroach_compiler",
            "gazelle:go_grpc_compilers @com_github_cockroachdb_cockroach//pkg/cmd/protoc-gen-gogoroach:protoc-gen-gogoroach_grpc_compiler",
            "gazelle:proto_import_prefix raft/v3",
        ],
        build_file_proto_mode = "default",
        importpath = "go.etcd.io/raft/v3",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:io_etcd_go_raft_v3.patch",
        ],
        replace = "github.com/cockroachdb/raft/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Lv0iiCw5BB6y/loaOgeC75OwB9x6Hi1XbBuhU9eQFNg=",
        version = "v3.0.0-20230718062839-336c0ec27d85",
    )
    go_repository(
        name = "io_goji",
        build_file_proto_mode = "disable_global",
        importpath = "goji.io",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:uIssv/elbKRLznFUy3Xj4+2Mz/qKhek/9aZQDUMae7c=",
        version = "v2.0.2+incompatible",
    )
    go_repository(
        name = "io_gorm_driver_postgres",
        build_file_proto_mode = "disable_global",
        importpath = "gorm.io/driver/postgres",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:oVLmefGqBTlgeEVG6LKnH6krOlo4TZ3Q/jIK21KUMlw=",
        version = "v1.3.5",
    )
    go_repository(
        name = "io_gorm_gorm",
        build_file_proto_mode = "disable_global",
        importpath = "gorm.io/gorm",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:TnlF26wScKSvknUC/Rn8t0NLLM22fypYBlvj1+aH6dM=",
        version = "v1.23.5",
    )
    go_repository(
        name = "io_k8s_api",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/api",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ISu3tD/jRhYfSW8jI/Q1e+lRxkR7w9UwQEZ7FgslrwY=",
        version = "v0.22.1",
    )
    go_repository(
        name = "io_k8s_apiextensions_apiserver",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/apiextensions-apiserver",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:WDZWkPcbgvchEdDd7ysL21GGPx3UKZQLDZXEkevT6n4=",
        version = "v0.17.3",
    )
    go_repository(
        name = "io_k8s_apimachinery",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/apimachinery",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DTARnyzmdHMz7bFWFDDm22AM4pLWTQECMpRTFu2d2OM=",
        version = "v0.22.1",
    )
    go_repository(
        name = "io_k8s_apiserver",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/apiserver",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:NnVriMMOpqQX+dshbDoZixqmBhfgrPk2uOh2fzp9vHE=",
        version = "v0.20.6",
    )
    go_repository(
        name = "io_k8s_client_go",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/client-go",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jW0ZSHi8wW260FvcXHkIa0NLxFBQszTlhiAVsU5mopw=",
        version = "v0.22.1",
    )
    go_repository(
        name = "io_k8s_code_generator",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/code-generator",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:q/hDMk2cvFzSxol7k/VA1qCssR7VSMXHQHhzuX29VJ8=",
        version = "v0.17.3",
    )
    go_repository(
        name = "io_k8s_component_base",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/component-base",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:G0inASS5vAqCpzs7M4Sp9dv9d0aElpz39zDHbSB4f4g=",
        version = "v0.20.6",
    )
    go_repository(
        name = "io_k8s_cri_api",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/cri-api",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iXX0K2pRrbR8yXbZtDK/bSnmg/uSqIFiVJK1x4LUOMc=",
        version = "v0.20.6",
    )
    go_repository(
        name = "io_k8s_gengo",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/gengo",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:sAvhNk5RRuc6FNYGqe7Ygz3PSo/2wGWbulskmzRX8Vs=",
        version = "v0.0.0-20200413195148-3a45101e95ac",
    )
    go_repository(
        name = "io_k8s_klog",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/klog",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Pt+yjF5aB1xDSVbau4VsWe+dQNzA0qv1LlXdC2dF6Q8=",
        version = "v1.0.0",
    )
    go_repository(
        name = "io_k8s_klog_v2",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/klog/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:R2HDMDJsHVTHA2n4RjwbeYXdOcBymXdX/JRb1v0VGhE=",
        version = "v2.10.0",
    )
    go_repository(
        name = "io_k8s_kube_openapi",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/kube-openapi",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:KLHHjkdQFomZy8+06csTWZ0m1343QqxZhR2LJ1OxCYM=",
        version = "v0.0.0-20210421082810-95288971da7e",
    )
    go_repository(
        name = "io_k8s_kubernetes",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/kubernetes",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:qTfB+u5M92k2fCCCVP2iuhgwwSOv1EkAkvQY1tQODD8=",
        version = "v1.13.0",
    )
    go_repository(
        name = "io_k8s_sigs_apiserver_network_proxy_konnectivity_client",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/apiserver-network-proxy/konnectivity-client",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4uqm9Mv+w2MmBYD+F4qf/v6tDFUdPOk29C095RbU5mY=",
        version = "v0.0.15",
    )
    go_repository(
        name = "io_k8s_sigs_structured_merge_diff",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/structured-merge-diff",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zD2IemQ4LmOcAumeiyDWXKUI2SO0NYDe3H6QGvPOVgU=",
        version = "v1.0.1-0.20191108220359-b1b620dd3f06",
    )
    go_repository(
        name = "io_k8s_sigs_structured_merge_diff_v2",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/structured-merge-diff/v2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:I0h4buiCqDtPztO3NOiyoNMtqSIfld49D4Wj3UBXYZA=",
        version = "v2.0.1",
    )
    go_repository(
        name = "io_k8s_sigs_structured_merge_diff_v4",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/structured-merge-diff/v4",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Hr/htKFmJEbtMgS/UD0N+gtgctAqz81t3nu+sPzynno=",
        version = "v4.1.2",
    )
    go_repository(
        name = "io_k8s_sigs_yaml",
        build_file_proto_mode = "disable_global",
        importpath = "sigs.k8s.io/yaml",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:kr/MCeFWJWTwyaHoR9c8EjH9OumOmoF9YGiZd7lFm/Q=",
        version = "v1.2.0",
    )
    go_repository(
        name = "io_k8s_utils",
        build_file_proto_mode = "disable_global",
        importpath = "k8s.io/utils",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:imL9YgXQ9p7xmPzHFm/vVd/cF78jad+n4wK1ABwYtMM=",
        version = "v0.0.0-20210707171843-4b05e18ac7d9",
    )
    go_repository(
        name = "io_opencensus_go",
        build_file_proto_mode = "disable_global",
        importpath = "go.opencensus.io",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:y73uSU6J157QMP2kn2r30vwW1A2W2WFwSCGnAVxeaD0=",
        version = "v0.24.0",
    )
    go_repository(
        name = "io_opencensus_go_contrib_exporter_prometheus",
        build_file_proto_mode = "disable_global",
        importpath = "contrib.go.opencensus.io/exporter/prometheus",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0QfIkj9z/iVZgK31D9H9ohjjIDApI2GOPScCKwxedbs=",
        version = "v0.4.0",
    )
    go_repository(
        name = "io_opentelemetry_go_auto_sdk",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/auto/sdk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jXsnJ4Lmnqd11kwkBV2LgLoFMZKizbCi5fNZ/ipaZ64=",
        version = "v1.2.1",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_detectors_gcp",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/detectors/gcp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:62yY3dT7/ShwOxzA0RsKRgshBmfElKI4d/Myu2OxDFU=",
        version = "v1.43.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_instrumentation_google_golang_org_grpc_otelgrpc",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0Qx7VGBacMm9ZENQ7TnNObTYI4ShC+lHI16seduaxZo=",
        version = "v0.68.0",
    )
    go_repository(
        name = "io_opentelemetry_go_contrib_instrumentation_net_http_otelhttp",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CqXxU8VOmDefoh0+ztfGaymYbhdB/tT3zs79QaZTNGY=",
        version = "v0.68.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mYIM03dnh5zfN7HautFE4ieIig9amkNANT+xcVxAj9I=",
        version = "v1.43.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_otlp_otlptrace",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8dTSyAm+Zdtr6tHPJZBLwbIylteO05bfQTfP5gIIg24=",
        version = "v1.0.0-RC3.0.20210907151655-df2bdbbadb26",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_exporters_otlp_otlptrace_otlptracegrpc",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:F3cdr1An+QUPcj4SQrS92fEWV4A31jWFHCLpbiDUdCo=",
        version = "v1.0.0-RC3",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_metric",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/metric",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:d7638QeInOnuwOONPp4JAOGfbCEpYb+K6DVWvdxGzgM=",
        version = "v1.43.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/sdk",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:pi5mE86i5rTeLXqoF/hhiBtUNcrAGHLKQdhg4h4V9Dg=",
        version = "v1.43.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_sdk_metric",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/sdk/metric",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:S88dyqXjJkuBNLeMcVPRFXpRw2fuwdvfCGLEo89fDkw=",
        version = "v1.43.0",
    )
    go_repository(
        name = "io_opentelemetry_go_otel_trace",
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/otel/trace",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BkNrHpup+4k4w+ZZ86CZoHHEkohws8AY+WTX09nk+3A=",
        version = "v1.43.0",
    )
    go_repository(
        name = "io_opentelemetry_go_proto_otlp",
        build_directives = [
            "gazelle:resolve go go github.com/golang/protobuf/descriptor @com_github_golang_protobuf//descriptor:go_default_library_gen",
        ],
        build_file_proto_mode = "disable_global",
        importpath = "go.opentelemetry.io/proto/otlp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:C0g6TWmQYvjKRnljRULLWUVJGy8Uvu0NEL/5frY2/t4=",
        version = "v0.9.0",
    )
    go_repository(
        name = "io_rsc_binaryregexp",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/binaryregexp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:HfqmD5MEmC0zvwBuF187nq9mdnXjXsSivRiXN7SmRkE=",
        version = "v0.2.0",
    )
    go_repository(
        name = "io_rsc_pdf",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/pdf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:k1MczvYDUvJBe93bYd7wrZLLUEcLZAuF824/I4e5Xr4=",
        version = "v0.1.1",
    )
    go_repository(
        name = "io_rsc_quote_v3",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/quote/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9JKUTTIUgS6kzR9mK1YuGKv6Nl+DijDNIc0ghT58FaY=",
        version = "v3.1.0",
    )
    go_repository(
        name = "io_rsc_sampler",
        build_file_proto_mode = "disable_global",
        importpath = "rsc.io/sampler",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7uVkIFmeBqHfdjD+gZwtXXI+RODJ2Wc4O7MPEh/QiW4=",
        version = "v1.3.0",
    )
    go_repository(
        name = "io_vitess_vitess",
        build_file_proto_mode = "disable_global",
        importpath = "vitess.io/vitess",
        replace = "github.com/cockroachdb/vitess",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:8htEd1lLILqfjKardWfKKGgXVCs0WmcgEj9cXnmcuos=",
        version = "v0.0.0-20210218160543-54524729cc82",
    )
    go_repository(
        name = "org_bazil_fuse",
        build_file_proto_mode = "disable_global",
        importpath = "bazil.org/fuse",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:SRsZGA7aFnCZETmov57jwPrWuTmaZK6+4R4v5FUe1/c=",
        version = "v0.0.0-20200407214033-5883e5a4b512",
    )
    go_repository(
        name = "org_collectd",
        build_file_proto_mode = "disable_global",
        importpath = "collectd.org",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:iNBHGw1VvPJxH2B6RiFWFZ+vsjo1lCdRszBeOuwGi00=",
        version = "v0.3.0",
    )
    go_repository(
        name = "org_golang_google_api",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/api",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DJSXnV6An+NhJ1J+GWtoF2nHEuqB1VNoTfnIbjNvwD4=",
        version = "v0.176.1",
    )
    go_repository(
        name = "org_golang_google_appengine",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/appengine",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:IhEN5q69dyKagZPYMSdIjS2HqprW324FRQZJcGqPAsM=",
        version = "v1.6.8",
    )
    go_repository(
        name = "org_golang_google_cloud",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/cloud",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Cpp2P6TPjujNoC5M2KHY6g7wfyLYfIWRZaSdIKfDasA=",
        version = "v0.0.0-20151119220103-975617b05ea8",
    )
    go_repository(
        name = "org_golang_google_genproto",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9+tzLLstTlPTRyJTh+ah5wIMsBW5c4tQwGTN3thOW9Y=",
        version = "v0.0.0-20240213162025-012b6fc9bca9",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_api",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto/googleapis/api",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:yQugLulqltosq0B/f8l4w9VryjV+N/5gcW0jQ3N8Qec=",
        version = "v0.0.0-20260414002931-afd174a4e478",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_bytestream",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto/googleapis/bytestream",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wBkzraZsSqhj1M4L/nMrljUU6XasJkgHvUsq8oRGwF0=",
        version = "v0.0.0-20240325203815-454cdb8f5daa",
    )
    go_repository(
        name = "org_golang_google_genproto_googleapis_rpc",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/genproto/googleapis/rpc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wT2n40TBqFY6wiwazVK9/iTWbsQrgk5ZfCSVFLO9LQA=",
        version = "v0.0.0-20260406210006-6f92a3bedf2d",
    )
    go_repository(
        name = "org_golang_google_grpc",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/grpc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Xr6m2WmWZLETvUNvIUmeD5OAagMw3FiKmMlTdViWsHM=",
        version = "v1.80.0",
    )
    go_repository(
        name = "org_golang_google_grpc_cmd_protoc_gen_go_grpc",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/grpc/cmd/protoc-gen-go-grpc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:M1YKkFIboKNieVO5DLUEVzQfGwJD30Nv2jfUgzb5UcE=",
        version = "v1.1.0",
    )
    go_repository(
        name = "org_golang_google_grpc_examples",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/grpc/examples",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:CJP00gEaaYNJuaTXEg456rWNC1oUOfiAiUjuwyAhEmM=",
        version = "v0.0.0-20210324172016-702608ffae4d",
    )
    go_repository(
        name = "org_golang_google_protobuf",
        build_file_proto_mode = "disable_global",
        importpath = "google.golang.org/protobuf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fV6ZwhNocDyBLK0dj+fg8ektcVegBBuEolpbTQyBNVE=",
        version = "v1.36.11",
    )
    go_repository(
        name = "org_golang_x_arch",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/arch",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Vsc61gop4hfHdzQNolo6Fi/sw7TnJ2yl3ZR4i7bYirs=",
        version = "v0.0.0-20180920145803-b19384d3c130",
    )
    go_repository(
        name = "org_golang_x_crypto",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/crypto",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zO47/JPrL6vsNkINmLoo/PH1gcxpls50DNogFvB5ZGI=",
        version = "v0.50.0",
    )
    go_repository(
        name = "org_golang_x_exp",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/exp",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:W3F4c+6OLc6H2lb//N1q4WpJkhzJCK5J6kUi1NTVXfM=",
        version = "v0.0.0-20260410095643-746e56fc9e2f",
    )
    go_repository(
        name = "org_golang_x_exp_typeparams",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/exp/typeparams",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Jw5wfR+h9mnIYH+OtGT2im5wV1YGGDora5vTv/aa5bE=",
        version = "v0.0.0-20221208152030-732eee02a75a",
    )
    go_repository(
        name = "org_golang_x_image",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/image",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:skVYidAEVKgn8lZ602XO75asgXBgLj9G/FE3RbuPFww=",
        version = "v0.39.0",
    )
    go_repository(
        name = "org_golang_x_lint",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/lint",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VLliZ0d+/avPrXXH+OakdXhpJuEoBZuwh1m2j7U6Iug=",
        version = "v0.0.0-20210508222113-6edffad5e616",
    )
    go_repository(
        name = "org_golang_x_mobile",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/mobile",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4+4C/Iv2U4fMZBiMCc98MG1In4gJY5YRhtpDNeDeHWs=",
        version = "v0.0.0-20190719004257-d2bd2a29d028",
    )
    go_repository(
        name = "org_golang_x_mod",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/mod",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Ww1D637e6Pg+Zb2KrWfHQUnH2dQRLBQyAtpr/haaJeM=",
        version = "v0.35.0",
    )
    go_repository(
        name = "org_golang_x_net",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/net",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:d+qAbo5L0orcWAr0a9JweQpjXF19LMXJE8Ey7hwOdUA=",
        version = "v0.53.0",
    )
    go_repository(
        name = "org_golang_x_oauth2",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/oauth2",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:peZ/1z27fi9hUOFCAZaHyrpWG5lwe0RJEEEeH0ThlIs=",
        version = "v0.36.0",
    )
    go_repository(
        name = "org_golang_x_perf",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/perf",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:rOAIsN39Q2RCgXyAHfrbTD5Za05y4mlAyjpMYuWkd+c=",
        version = "v0.0.0-20260409210113-8e83ce0f7b1c",
    )
    go_repository(
        name = "org_golang_x_sync",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/sync",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:e0PTpb7pjO8GAtTs2dQ6jYa5BWYlMuX047Dco/pItO4=",
        version = "v0.20.0",
    )
    go_repository(
        name = "org_golang_x_sys",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/sys",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Rlag2XtaFTxp19wS8MXlJwTvoh8ArU6ezoyFsMyCTNI=",
        version = "v0.43.0",
    )
    go_repository(
        name = "org_golang_x_telemetry",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/telemetry",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:efT73AJZfAAUV7SOip6pWGkwJDzIGiKBZGVzHYa+ve4=",
        version = "v0.0.0-20260409153401-be6f6cb8b1fa",
    )
    go_repository(
        name = "org_golang_x_term",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/term",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UiKe+zDFmJobeJ5ggPwOshJIVt6/Ft0rcfrXZDLWAWY=",
        version = "v0.42.0",
    )
    go_repository(
        name = "org_golang_x_text",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/text",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:JfKh3XmcRPqZPKevfXVpI1wXPTqbkE5f7JA92a55Yxg=",
        version = "v0.36.0",
    )
    go_repository(
        name = "org_golang_x_time",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/time",
        replace = "github.com/cockroachdb/x-time",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:fL+zfQ88TclMTv0pWlHhe3z+rZvb9CpyCp+fFQSN2Ys=",
        version = "v0.3.1-0.20230525123634-71747adb5d5c",
    )
    go_repository(
        name = "org_golang_x_tools",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/tools",
        patch_args = ["-p1"],
        patches = [
            "@com_github_cockroachdb_cockroach//build/patches:org_golang_x_tools.patch",
        ],
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:UP4ajHPIcuMjT1GqzDWRlalUEoY+uzoZKnhOjbIPD2c=",
        version = "v0.44.0",
    )
    go_repository(
        name = "org_golang_x_tools_go_expect",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/tools/go/expect",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jpBZDwmgPhXsKZC6WhL20P4b/wmnpsEAGHaNy0n/rJM=",
        version = "v0.1.1-deprecated",
    )
    go_repository(
        name = "org_golang_x_tools_go_packages_packagestest",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/tools/go/packages/packagestest",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:1h2MnaIAIXISqTFKdENegdpAgUXz6NrPEsbIeWaBRvM=",
        version = "v0.1.1-deprecated",
    )
    go_repository(
        name = "org_golang_x_xerrors",
        build_file_proto_mode = "disable_global",
        importpath = "golang.org/x/xerrors",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:noIWHXmPHxILtqtCOPIhSt0ABwskkZKjD3bXGnZGpNY=",
        version = "v0.0.0-20240903120638-7835f813f4da",
    )
    go_repository(
        name = "org_gonum_v1_gonum",
        build_file_proto_mode = "disable_global",
        importpath = "gonum.org/v1/gonum",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VbpOemQlsSMrYmn7T2OUvQ4dqxQXU+ouZFQsZOx50z4=",
        version = "v0.17.0",
    )
    go_repository(
        name = "org_gonum_v1_netlib",
        build_file_proto_mode = "disable_global",
        importpath = "gonum.org/v1/netlib",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:jRyg0XfpwWlhEV8mDfdNGBeSJM2fuyh9Yjrnd8kF2Ts=",
        version = "v0.0.0-20190331212654-76723241ea4e",
    )
    go_repository(
        name = "org_gonum_v1_plot",
        build_file_proto_mode = "disable_global",
        importpath = "gonum.org/v1/plot",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:dnifSs43YJuNMDzB7v8wV64O4ABBHReuAVAoBxqBqS4=",
        version = "v0.10.1",
    )
    go_repository(
        name = "org_modernc_cc",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/cc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:nPibNuDEx6tvYrUAtvDTTw98rx5juGsa5zuDnKwEEQQ=",
        version = "v1.0.0",
    )
    go_repository(
        name = "org_modernc_cc_v3",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/cc/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:BzShpwCAP7TWzFppM4k2t03RhXhgYqaibROWkrWq7lE=",
        version = "v3.35.22",
    )
    go_repository(
        name = "org_modernc_ccgo_v3",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/ccgo/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:hqlCzNJTXLrhS70y1PqWckrF9x1btSQRC7JFuQcBg5c=",
        version = "v3.15.13",
    )
    go_repository(
        name = "org_modernc_ccorpus",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/ccorpus",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:YOmQBBzE8GC/puUx76D5j/gJYIZQsydrh6VMJVfXF0M=",
        version = "v1.11.4",
    )
    go_repository(
        name = "org_modernc_golex",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/golex",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:wWpDlbK8ejRfSyi0frMyhilD3JBvtcx2AdGDnU+JtsE=",
        version = "v1.0.0",
    )
    go_repository(
        name = "org_modernc_httpfs",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/httpfs",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:AAgIpFZRXuYnkjftxTAZwMIiwEqAfk8aVB2/oA6nAeM=",
        version = "v1.0.6",
    )
    go_repository(
        name = "org_modernc_libc",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/libc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:DAHvwGoVRDZs5iJXnX9RJrgXSsorupCWmJ2ac964Owk=",
        version = "v1.14.5",
    )
    go_repository(
        name = "org_modernc_mathutil",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/mathutil",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:ij3fYGe8zBF4Vu+g0oT7mB06r8sqGWKuJu1yXeR4by8=",
        version = "v1.4.1",
    )
    go_repository(
        name = "org_modernc_memory",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/memory",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XRch8trV7GgvTec2i7jc33YlUI0RKVDBvZ5eZ5m8y14=",
        version = "v1.0.5",
    )
    go_repository(
        name = "org_modernc_opt",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/opt",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:/0RX92k9vwVeDXj+Xn23DKp2VJubL7k8qNffND6qn3A=",
        version = "v0.1.1",
    )
    go_repository(
        name = "org_modernc_sqlite",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/sqlite",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:Jt5P3k80EtDBWaq1beAxnWW+5MdHXbZITujnRS7+zWg=",
        version = "v1.14.6",
    )
    go_repository(
        name = "org_modernc_strutil",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/strutil",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:xv+J1BXY3Opl2ALrBwyfEikFAj8pmqcpnfmuwUwcozs=",
        version = "v1.1.1",
    )
    go_repository(
        name = "org_modernc_tcl",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/tcl",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:B/zzEYjINeaki38KcIqdQRQx7W3WE7TkrlTwGnbm2II=",
        version = "v1.11.0",
    )
    go_repository(
        name = "org_modernc_token",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/token",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:a0jaWiNMDhDUtqOj09wvjWWAqd3q7WpBulmL9H2egsk=",
        version = "v1.0.0",
    )
    go_repository(
        name = "org_modernc_xc",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/xc",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:7ccXrupWZIS3twbUGrtKmHS2DXY6xegFua+6O3xgAFU=",
        version = "v1.0.0",
    )
    go_repository(
        name = "org_modernc_z",
        build_file_proto_mode = "disable_global",
        importpath = "modernc.org/z",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:4RWULo1Nvaq5ZBhbLe74u8p6tV4Mmm0ZrPBXYPm/xjM=",
        version = "v1.3.0",
    )
    go_repository(
        name = "org_mongodb_go_mongo_driver",
        build_file_proto_mode = "disable_global",
        importpath = "go.mongodb.org/mongo-driver",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9nOVLGDfOaZ9R0tBumx/BcuqkbFpyTCU2r/Po7A2azI=",
        version = "v1.5.1",
    )
    go_repository(
        name = "org_mozilla_go_pkcs7",
        build_file_proto_mode = "disable_global",
        importpath = "go.mozilla.org/pkcs7",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:A/5uWzF44DlIgdm/PQFwfMkW0JX+cIcQi/SwLAmZP5M=",
        version = "v0.0.0-20200128120323-432b2356ecb1",
    )
    go_repository(
        name = "org_uber_go_atomic",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/atomic",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:9qC72Qh0+3MqyJbAn8YU5xVq1frD8bn3JtD2oXtafVQ=",
        version = "v1.10.0",
    )
    go_repository(
        name = "org_uber_go_goleak",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/goleak",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:2K3zAYmnTNqV73imy9J1T3WC+gmCePx2hEGkimedGto=",
        version = "v1.3.0",
    )
    go_repository(
        name = "org_uber_go_multierr",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/multierr",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:zaiO/rmgFjbmCXdSYJWQcdvOCsthmdaHfr3Gm2Kx4Ec=",
        version = "v1.7.0",
    )
    go_repository(
        name = "org_uber_go_tools",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/tools",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:0mgffUl7nfd+FpvXMVz4IDEaUSmT1ysygQC7qYo7sG4=",
        version = "v0.0.0-20190618225709-2cfd321de3ee",
    )
    go_repository(
        name = "org_uber_go_zap",
        build_file_proto_mode = "disable_global",
        importpath = "go.uber.org/zap",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:mZQZefskPPCMIBCSEH0v2/iUqqLrYtaeqwD6FUGUnFE=",
        version = "v1.19.0",
    )
    go_repository(
        name = "tech_einride_go_aip",
        build_file_proto_mode = "disable_global",
        importpath = "go.einride.tech/aip",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:XfV+NQX6L7EOYK11yoHHFtndeaWh3KbD9/cN/6iWEt8=",
        version = "v0.66.0",
    )
    go_repository(
        name = "tools_gotest",
        build_file_proto_mode = "disable_global",
        importpath = "gotest.tools",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:VsBPFP1AI068pPrMxtb/S8Zkgf9xEmTLJjfM+P5UIEo=",
        version = "v2.2.0+incompatible",
    )
    go_repository(
        name = "tools_gotest_v3",
        build_file_proto_mode = "disable_global",
        importpath = "gotest.tools/v3",
        # TODO: mirror this repo (to fix, run `./dev generate bazel --mirror`)
        sum = "h1:I0DwBVMGAx26dttAj1BtJLAkVGncrkkUXfJLC4Flt/I=",
        version = "v3.2.0",
    )
