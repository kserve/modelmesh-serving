# Release Process

## Create an Issue to Track the Release Process

To better coordinate the release process create an issue in the
[kserve/modelmesh-serving](https://github.com/kserve/modelmesh-serving/issues/new?title=release:)
to keep track of all the steps that need to be done. Many of the checklists below
are intended to be copied into the issue description so that the completed steps
can be checked of while the release process is moving forward.

Also utilize the [bi-weekly KServe community call](https://docs.google.com/document/d/1KZUURwr9MnHXqHA08TFbfVbM8EAJSJjmaMhnvstvi-k/edit?usp=sharing)
to coordinate the release between KServe and ModelMesh or reach out to `@dsun`
on the [`#kserve` Kubeflow Slack channel](https://kubeflow.slack.com/archives/CH6E58LNP).

## Prepare the Release

Before starting the actual release process, make sure that the features and bug
fixes that were designated to be part of the release are completed and fully tested.

Update the `go` dependency to `github.com/kserve/kserve` in `go.mod` and run
`go mod tidy`. Since the KServe and ModelMesh releases are aligned, there should
already be a `v...-rc0` (or `-rc1`) of KServe before the ModelMesh release process
is started. Use this opportunity to update other `go` dependencies as well.
The dependencies should be updated on the `main` branch, before creating a new
release branch.

Update the CRDs under `config/crd/bases` with the CRDs from the `kserve/kserve`
repository (`https://github.com/kserve/kserve/tree/master/config/crd`) using their
latest pre-release version:

- [ ] `config/crd/bases/serving.kserve.io_inferenceservices.yaml`
- [ ] `config/crd/bases/serving.kserve.io_clusterservingruntimes.yaml`
- [ ] `config/crd/bases/serving.kserve.io_servingruntimes.yaml`

## Create Release Branches

A release branch should be substantially _feature complete_ with respect to the
intended release. Code that is committed to `main` may be merged or cherry-picked
on to a release branch, but code that is directly committed to the release branch
should be solely applicable to that release (and should not be committed back to
`main`). In general, unless you're committing code that only applies to the release
stream (for example, temporary hotfixes, backported security fixes, or image hashes),
you should commit to `main` and then merge or cherry-pick to the release branch.

Create a _release branch_ from `main` in the form of `release-${MAJOR}.${MINOR}`.
Release branches serve several purposes:

1. They allow a release wrangler or other developers to focus on a release without
   interrupting development on `main`,
2. They allow developers to track the development of a release before a release
   candidate is declared,
3. They simplify back porting critical bug fixes to a patch level release for a
   particular release stream (e.g., producing a `v0.6.1` from `release-0.6`),
   when appropriate.

These 5 repositories need a (new) `release-*` branch:

- [ ] [`modelmesh`](https://github.com/kserve/modelmesh/branches)
- [ ] [`modelmesh-minio-examples`](https://github.com/kserve/modelmesh-minio-examples/branches)
- [ ] [`modelmesh-runtime-adapter`](https://github.com/kserve/modelmesh-runtime-adapter/branches)
- [ ] [`modelmesh-serving`](https://github.com/kserve/modelmesh-serving/branches)
- [ ] [`rest-proxy`](https://github.com/kserve/rest-proxy/branches)

## Update Release Tags

It's generally a good idea to search the entire repo for strings of the old version
number and replace them with the new, keeping in mind conflicts with other library
version numbers. Most IDEs support searching an entire project using `command`-`shift`-`f`.

Some of the steps below need to be performed at least twice:

- at least once for the release candidate(s) (`v0.10.0-rc0`, `v0.10.0-rc1`, ...) and
- once more for the actual release (`v0.10.0`).

While creating a pre-release is not technically required, it is considered good
practice. It allows other stakeholders to deploy and test the designated release,
while keeping the door open to address any bugs before publishing a final release.
This is especially useful when testing the new ModelMesh release in conjunction
with KServe.

1. Create new (pre-)release tags (`v...-rc0`) in these repositories:

   - [ ] [`modelmesh`](https://github.com/kserve/modelmesh/releases)
   - [ ] [`modelmesh-minio-examples`](https://github.com/kserve/modelmesh-runtime-adapter/releases)
   - [ ] [`modelmesh-runtime-adapter`](https://github.com/kserve/modelmesh-runtime-adapter/releases)
   - [ ] [`rest-proxy`](https://github.com/kserve/rest-proxy/releases)

   using the newly created `release-*` branches as target. This can be done by
   creating a draft release using the GitHub web interface and checking the
   "Set as a pre-release" option. The newly created tag should trigger the GitHub
   action to push the respective (pre-)release container images to DockerHub which
   are needed in the next step.

2. Verify image tags were pushed to [DockerHub](https://hub.docker.com/u/kserve):

   - [ ] [kserve/modelmesh](https://hub.docker.com/r/kserve/modelmesh/tags)
   - [ ] [kserve/modelmesh-minio-examples](https://hub.docker.com/r/kserve/modelmesh-minio-examples/tags)
   - [ ] [kserve/modelmesh-runtime-adapter](https://hub.docker.com/r/kserve/modelmesh-runtime-adapter/tags)
   - [ ] [kserve/rest-proxy](https://hub.docker.com/r/kserve/rest-proxy/tags)

3. In this `modelmesh-serving` repository, update the container image tags to
   the corresponding release versions for:

   - `kserve/modelmesh`
   - `kserve/modelmesh-controller`
   - `kserve/modelmesh-minio-examples`
   - `kserve/modelmesh-runtime-adapter`
   - `kserve/rest-proxy`

   The version tags should be updated in the following files:

   - [ ] `config/default/config-defaults.yaml`:
     - [ ] `kserve/modelmesh`
     - [ ] `kserve/rest-proxy`
     - [ ] `kserve/modelmesh-runtime-adapter`
   - [ ] `config/dependencies/quickstart.yaml`:
     - [ ] `kserve/modelmesh-minio-examples`
   - [ ] `config/manager/kustomization.yaml`: edit the `newTag`
   - [ ] `docs/component-versions.md`: update the version and component versions
   - [ ] `docs/install/install-script.md`: update the `RELEASE` variable in the
         `Installation` section to the new `release-*` branch name
   - [ ] `docs/quickstart.md`: update the `RELEASE` variable in the
         _"Get the latest release"_ section to the new `release-*` branch name
   - [ ] `scripts/setup_user_namespaces.sh`: change the `modelmesh_release` version

   You can copy the checklist above into the PR description in the next step.

4. Submit your PR to the `release-*` branch that was created earlier and wait for
   it to merge.

5. Update the following files in the `main` branch with the same versions as in the
   steps above, submit them in a PR to `main`, and wait for that PR to be merged:

   - [ ] `docs/component-versions.md`
   - [ ] `docs/quickstart.md`
   - [ ] `docs/install/install-script.md`
   - [ ] `scripts/setup_user_namespaces.sh`

## Generate Release Artifacts and Publish the Release

1. Generate the release manifests on the `release-*` branch:

   ```Shell
   kustomize build config/default > modelmesh.yaml
   kustomize build config/runtimes --load-restrictor LoadRestrictionsNone > modelmesh-runtimes.yaml
   cp config/dependencies/quickstart.yaml modelmesh-quickstart-dependencies.yaml
   ```

   If you see `Error: unknown flag: --load-restrictor` upgrade your `kustomize` version to 4.x.

2. Generate config archive on the `release-*` branch. The scriptlet below automatically
   determines the release version and chooses the version of the `tar` command for
   either Linux or macOS. Verify the correct release `VERSION` was found.

   ```Shell
   VERSION=$( grep -o -E "newTag: .*$" config/manager/kustomization.yaml | sed 's/newTag: //' )
   TAR_FILE="config-${VERSION}.tar.gz"

   echo "Release: ${VERSION}"

   if $(tar --version | grep -q 'bsd'); then
     tar -zcvf ${TAR_FILE} -s /config/config-${VERSION}/ config/;
   else
     tar -zcvf ${TAR_FILE} config/ --transform s/config/config-${VERSION}/;
   fi
   ```

3. Create a new tag on the `release-*` branch and push it to GitHub using the commands
   below, or, create a new tag in the next step using the GitHub UI. The new
   `kserve/modelmesh-controller` image will be published via GitHub Actions.

   ```Shell
   git tag $VERSION
   git push upstream $VERSION

   echo https://github.com/kserve/modelmesh-serving/releases/new?tag=${VERSION}
   ```

4. Create the new release in the GitHub UI from the `release-*` branch (or from the
   tag created in the previous step). Enter the release tag value (e.g. `v0.10.0`) in
   the "Release title" field and upload the generated installation manifests ("Release assets")
   in the "Attach binaries ..." section. Click the "Generate release notes" button which
   will generate the release description.

   **Note**, if you generated a pre-release (e.g. `v0.10.0-rc0`) then copy the release
   notes from that and remove them from the pre-release description and revise accordingly.

   https://github.com/kserve/modelmesh-serving/releases/new

5. Compare the release and release artifacts to those of previous releases to make
   sure nothing was missed.

6. Once the release as been published (a new tag has been pushed), verify the check
   results by clicking on the check mark (✓) next to the latest commit on the `release-*`
   branch.

7. Verify that the newly released version of the
   [`modelmesh-controller`](https://hub.docker.com/r/kserve/modelmesh-controller/tags)
   was pushed to DockerHub.

## Update the KServe Helm Charts

Fork and clone the [`kserve/kserve`](https://github.com/kserve/kserve) repository
and update all references to the old ModelMesh versions. At the time of the `v0.10.0`
release the following files needed to be [updated](https://github.com/kserve/kserve/pull/2645).

- `charts/kserve-resources/values.yaml`
- `hack/install_kserve_mm.sh`

Furthermore, the `helm` charts under `charts/kserve-resources/templates` which are
used to install ModelMesh as part of KServe need to be updated with the changes
in the respective manifests from the `kserve/modelmesh-serving` repository found
in the `config` folder.

For reference, for the `v0.10.0` release the following charts in the `kserve` repo
had to be [updated](https://github.com/kserve/kserve/pull/2645):

- `charts/kserve-resources/templates/clusterrole.yaml`
- `charts/kserve-resources/templates/clusterservingruntimes.yaml`
- `charts/kserve-resources/templates/configmap.yaml`
- `charts/kserve-resources/templates/deployment.yaml`

For the `v0.9.0` release the following charts had to be [updated](https://github.com/kserve/kserve/pull/2315):

- `charts/kserve/crds/serving.kserve.io_predictor.yaml`
- `charts/kserve/templates/clusterrole.yaml`
- `charts/kserve/templates/configmap.yaml`
- `charts/kserve/templates/deployment.yaml`
- `charts/kserve/templates/networkpolicy.yaml`
- `charts/kserve/templates/rolebinding.yaml`
- `charts/kserve/templates/servingruntimes.yaml`

## Update the KServe Website

In the `kserve/website` repository, update all reference to the previous ModelMesh
release. As of `v0.10.0`, `docs/admin/modelmesh.md` was the only Markdown file to
be [updated](https://github.com/kserve/website/pull/214).

## Release Blog

Work with [Dan Sun](https://kubeflow.slack.com/archives/D04PVPHMN8K) on a joint
release blog.

For reference, here are a few examples of previous release blogs featuring ModelMesh:

- [v0.10.0](https://kserve.github.io/website/0.10/blog/articles/2023-02-05-KServe-0.10-release/#modelmesh-updates)
- [v0.9.0](https://kserve.github.io/website/0.9/blog/articles/2022-07-21-KServe-0.9-release/#inferenceservice-api-for-modelmesh)
- [v0.8.0](https://kserve.github.io/website/0.8/blog/articles/2022-02-18-KServe-0.8-release/#modelmesh-updates)
- [v0.7.0](https://kserve.github.io/website/0.7/blog/articles/2021-10-11-KServe-0.7-release)

And the corresponding PRs to illustrate the process and the participants:

- [v0.10.0 draft](https://github.com/kserve/website/pull/227#discussion_r1098917277)
- [v0.9.0 draft](https://github.com/kserve/website/pull/166)
- [v0.8.0 draft](https://github.com/kserve/website/pull/105)
- [v0.7.0 draft](https://github.com/kserve/website/pull/49)
