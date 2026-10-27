#!/bin/bash
#
# Packages the binaries built by the GitHub Actions build workflow into the
# release archives of qsock.
#
# Every build of the master branch uploads one artifact per target, this
# fetches the artifacts of the latest build with the gh CLI and packages each
# one of them into release/qsock_<os>_<arch>.tar.gz.

## ANSI Colors (FG & BG)
RED="$(printf '\033[31m')" GREEN="$(printf '\033[32m')" YELLOW="$(printf '\033[33m')"
WHITE="$(printf '\033[37m')"
RESET="$(printf '\e[0m')"

## Globals
BIN="qsock"
SRC_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_DIR="$SRC_ROOT/release"
WORKFLOW="main.yml"
BRANCH="master"
RUN_ID=""
ARTIFACT_DIR=""
REPO=""
STAGE_DIR=""
ERR_LOG="/dev/null"

print_status() {
    echo ${YELLOW}"[*] ${RESET}${1}"
}

print_progress() {
    [[ ! -z "${VERBOSE}" ]] && return
    echo -n ${YELLOW}"[*] ${RESET}${1}"
    n=${#1}
    printf %$((70 - $n))s | tr " " "."
}

print_warning() {
    echo ${YELLOW}"[!] ${RESET}${1}"
}

print_error() {
    echo ${RED}"[-] ${RESET}${1}"
}

print_fatal() {
    echo -e ${RED}"[!] $1\n${RESET}"
    exit 1
}

print_good() {
    echo ${GREEN}"[+] ${RESET}${1}"
}

print_verbose() {
    if [[ ! -z "${VERBOSE}" ]]; then
        echo ${WHITE}"[*] ${RESET}${1}"
    fi
}

print_ok() {
    [[ -z "${VERBOSE}" ]] && echo -e " [${GREEN}OK${RESET}]"
}

print_fail() {
    [[ -z "${VERBOSE}" ]] && echo -e " [${RED}FAIL${RESET}]"
}

print_skip() {
    [[ -z "${VERBOSE}" ]] && echo -e " [${YELLOW}SKIP${RESET}]"
}

must_exist() {
    for i in "$@"; do
        command -v $i &>$ERR_LOG || print_fatal "$i not installed! Exiting..."
    done
}

usage() {
    cat <<EOF
Usage: ${0##*/} [options]

Fetches the binaries built for the latest commit of a branch from the GitHub
Actions build workflow and packages them into ./release.

Options:
  -r RUN_ID    Package the given build run instead of the latest one.
  -b BRANCH    Branch whose latest build is packaged. (default: $BRANCH)
  -w WORKFLOW  Workflow holding the builds. (default: $WORKFLOW)
  -d DIR       Package an already downloaded artifact directory, the gh CLI
               is not needed with it.
  -o DIR       Directory the archives are written to. (default: ./release)
  -R REPO      Repository in <owner>/<name> form. (default: the git remote)
  -v           Verbose output.
  -h           Print this help and quit.
EOF
}

## Handle the signals
clean_exit() {
    [[ -n "$STAGE_DIR" && -d "$STAGE_DIR" ]] && rm -rf -- "$STAGE_DIR"
    return 0
}

exit_on_signal() {
    echo ""
    print_error "Script interrupted!"
    clean_exit
    exit 1
}

trap exit_on_signal SIGINT SIGTERM
trap clean_exit EXIT

# gh_run reads the latest completed run of the build workflow, the fields of
# the run are printed as a single tab separated line.
gh_run() {
    local fields="databaseId,headSha,status,conclusion"
    local query='first(.[] | select(.status == "completed")) | [.databaseId, .headSha, .conclusion] | @tsv'
    local args=(--workflow "$WORKFLOW" --branch "$BRANCH" --limit 20 --json "$fields" --jq "$query")
    [[ -n "$REPO" ]] && args+=(--repo "$REPO")
    gh run list "${args[@]}" 2>$ERR_LOG
}

# gh_download downloads every artifact of a run, each one of them lands in a
# directory named after the artifact.
gh_download() {
    local args=("$1" --dir "$2")
    [[ -n "$REPO" ]] && args+=(--repo "$REPO")
    gh run download "${args[@]}" &>$ERR_LOG
}

# package_artifact packages the binary of a downloaded artifact into a
# release archive. The artifacts carry the binary of a single target and are
# named after it, which is the name of the archive as well.
#
# $1 = <path/to/qsock_linux_amd64>
package_artifact() {
    local dir="$1" name target os arch suffix bin
    name="$(basename -- "$dir")"
    target="${name#${BIN}_}"
    os="${target%_*}"
    arch="${target##*_}"
    if [[ "$name" != "${BIN}_"*_* || -z "$os" || -z "$arch" ]]; then
        return 2
    fi

    bin="$(find "$dir" -type f -name "${BIN}*" -print -quit)"
    [[ -n "$bin" ]] || return 1

    # The binary is named after the platform it runs on inside the artifact,
    # the archive carries it under the name it is installed with.
    suffix=""
    [[ "$os" == "windows" ]] && suffix=".exe"
    cp -- "$bin" "$STAGE_DIR/${BIN}${suffix}" || return 1
    # An artifact is a zip archive, the mode bits of the binary do not
    # survive the round trip through it.
    chmod 755 "$STAGE_DIR/${BIN}${suffix}" || return 1

    print_verbose "Packaging $bin as ${name}.tar.gz..."
    tar -C "$STAGE_DIR" -czf "$RELEASE_DIR/${name}.tar.gz" "./${BIN}${suffix}" || return 1
    rm -f -- "$STAGE_DIR/${BIN}${suffix}"
    return 0
}

# print_checksums prints the digests of the archives, ready to be pasted
# under the release notes.
print_checksums() {
    local sum="sha256sum"
    command -v sha256sum &>$ERR_LOG || sum="shasum -a 256"
    (cd "$RELEASE_DIR" && echo -e "\n\`\`\`" && $sum *.tar.gz && echo '```')
}

while getopts ":r:b:w:d:o:R:vh" opt; do
    case $opt in
    r) RUN_ID="$OPTARG" ;;
    b) BRANCH="$OPTARG" ;;
    w) WORKFLOW="$OPTARG" ;;
    d) ARTIFACT_DIR="$OPTARG" ;;
    o) RELEASE_DIR="$OPTARG" ;;
    R) REPO="$OPTARG" ;;
    v) VERBOSE=1 ;;
    h)
        usage
        exit 0
        ;;
    \?)
        usage
        print_fatal "Unknown option -$OPTARG!"
        ;;
    :)
        usage
        print_fatal "Option -$OPTARG requires an argument!"
        ;;
    esac
done
[[ ! -z $VERBOSE ]] && ERR_LOG="/dev/stderr"

must_exist tar find
print_status "Initiating..."
print_status "Release Date: $(date)"

STAGE_DIR="$(mktemp -d)" || print_fatal "Failed creating the staging directory!"

if [[ -z "$ARTIFACT_DIR" ]]; then
    must_exist gh
    if [[ -z "$RUN_ID" ]]; then
        print_status "Looking up the latest $BRANCH build..."
        IFS=$'\t' read -r RUN_ID HEAD_SHA CONCLUSION < <(gh_run)
        [[ -n "$RUN_ID" ]] || print_fatal "No completed $WORKFLOW run found on $BRANCH! (is the gh CLI logged in?)"
        print_good "Build run: $RUN_ID ($HEAD_SHA)"
        [[ "$CONCLUSION" == "success" ]] ||
            print_warning "The run did not succeed ($CONCLUSION), some targets may be missing."
        # The binaries are the ones of the commit the run was started from,
        # which is not necessarily the one checked out here.
        LOCAL_SHA="$(git -C "$SRC_ROOT" rev-parse HEAD 2>$ERR_LOG)"
        if [[ -n "$LOCAL_SHA" && "$LOCAL_SHA" != "$HEAD_SHA" ]]; then
            print_warning "Local HEAD is ${LOCAL_SHA:0:7}, the binaries are built from ${HEAD_SHA:0:7}."
        fi
    fi

    ARTIFACT_DIR="$STAGE_DIR/artifacts"
    print_progress "Downloading the artifacts of run $RUN_ID"
    gh_download "$RUN_ID" "$ARTIFACT_DIR" && print_ok || {
        print_fail
        print_fatal "Failed downloading the artifacts of run $RUN_ID!"
    }
fi

[[ -d "$ARTIFACT_DIR" ]] || print_fatal "Could not find the artifact directory $ARTIFACT_DIR! Exiting..."
mkdir -p "$RELEASE_DIR" || print_fatal "Failed creating the release directory!"
# A leftover archive of a previous release is not part of this one.
rm -f -- "$RELEASE_DIR/${BIN}_"*.tar.gz

echo ""
COUNT=0
SKIPPED=0
for dir in "$ARTIFACT_DIR"/*/; do
    [[ -d "$dir" ]] || continue
    name="$(basename -- "$dir")"
    print_progress "Packaging $name"
    package_artifact "$dir"
    case $? in
    0)
        print_ok
        COUNT=$((COUNT + 1))
        ;;
    2)
        print_skip
        print_verbose "$name does not name a target, skipping it."
        SKIPPED=$((SKIPPED + 1))
        ;;
    *)
        print_fail
        SKIPPED=$((SKIPPED + 1))
        ;;
    esac
done

echo ""
[[ $COUNT -gt 0 ]] || print_fatal "No release archive could be packaged! Exiting..."
print_good "Packaged $COUNT archives into $RELEASE_DIR"
[[ $SKIPPED -eq 0 ]] || print_warning "Skipped $SKIPPED artifacts."
print_checksums
