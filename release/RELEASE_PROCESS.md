# Release Process

## Create an issue for release tracking

- Create an issue in [kserve/modelmesh-serving](https://github.com/kserve/modelmesh-serving)
- Label the issue with `priority p0`
- Label the issue with `kind process`

## Releasing KServe components

A release branch should be substantially _feature complete_ with respect to the
intended release. Code that is committed to `main` may be merged or cherry-picked
on to a release branch, but code that is directly committed to the release branch
should be solely applicable to that release (and should not be committed back to
`main`). In general, unless you're committing code that only applies to the release
stream (for example, temporary hotfixes, backported security fixes, or image hashes),
you should commit to `main` and then merge or cherry-pick to the release branch.

## Create a release branch

Create a _release branch_ from `main` in the form of `release-${MAJOR}.${MINOR}`.
Release branches serve several purposes:

1. They allow a release wrangler or other developers to focus on a release without interrupting development on `main`,
2. They allow developers to track the development of a release before a release candidate is declared,
3. They simplify back porting critical bug fixes to a patch level release for a particular release stream (e.g., producing a `v0.6.1` from `release-0.6`), when appropriate.

These 4 repositories need a (new) `release-*` branch:

- [`modelmesh`](https://github.com/kserve/modelmesh/branches)
- [`modelmesh-runtime-adapter`](https://github.com/kserve/modelmesh-runtime-adapter/branches)
- [`modelmesh-serving`](https://github.com/kserve/modelmesh-serving/branches)
- [`rest-proxy`](https://github.com/kserve/rest-proxy/branches)


## Publish the release

It's generally a good idea to search the repo or `control`-`shift`-`f` for strings
of the old version number and replace them with the new, keeping in mind conflicts
with other library version numbers.

Some of the steps below need to be performed at least twice:

- once for the release candidate(s) (`v0.10.0-rc0`, `v0.10.0-rc1`, ...) and
- once more for the actual release (`v0.10.0`).

1. Create new (pre-)release tags (v...`-rc0`) in these repositories:

   - [`modelmesh`](https://github.com/kserve/modelmesh/releases)
   - [`modelmesh-runtime-adapter`](https://github.com/kserve/modelmesh-runtime-adapter/releases)
   - [`rest-proxy`](https://github.com/kserve/rest-proxy/releases)

   using the newly created `release-*` branches as target. This can be done by
   creating a draft release using the GitHub web interface and checking the
   "Set as a pre-release" option. The newly created tag should trigger the GitHub
   action to push the respective (pre-)release container images to DockerHub which
   are needed in the next step.

2. Build and push a new tag for the `kserve/modelmesh-minio-examples` image. If
   there have been no changes since the last release, you can create a new tag
   from the previous release, e.g. creating a new `v0.10.0` tag from `latest`:

   ```shell
   docker manifest create kserve/modelmesh-minio-examples:v0.10.0 kserve/modelmesh-minio-examples:latest
   docker manifest push kserve/modelmesh-minio-examples:v0.10.0
   ```

3. In this `modelmesh-serving` repository, update the container image tags to
   the corresponding release versions for:

    - `kserve/modelmesh`
    - `kserve/modelmesh-runtime-adapter`
    - `kserve/modelmesh-controller`
    - `kserve/rest-proxy`

   The version tags should be updated in the following files:

   - [ ] `config/manager/kustomization.yaml`: edit the `newTag`
   - [ ] `config/default/config-defaults.yaml`: edit the `modelmesh`, `modelmesh-runtime-adapter`, and `rest-proxy` image tags
   - [ ] `config/dependencies/quickstart.yaml`: change the `kserve/modelmesh-minio-examples` image tag to use the pinned version
   - [ ] `docs/component-versions.md`: update the version and component versions
   - [ ] `docs/install/install-script.md`: update the `RELEASE` variable in the `Installation` section to the new release branch name.
   - [ ] `docs/quickstart.md`: update the `RELEASE` variable in the `Get the latest release` section to the new release branch name
   - [ ] `scripts/setup_user_namespaces.sh`: change the `modelmesh_release` version

4. Submit your PR to the `release-*` branch that was created earlier and wait for
   it to merge.

5. Update the following files in the `main` branch with the same versions as in the
   steps above, submit them in a PR to `main`, and wait for that PR to be merged:

   - `docs/component-versions.md`
   - `docs/quickstart.md`
   - `docs/install/install-script.md`
   - `scripts/setup_user_namespaces.sh`

6. Generate the release manifests:

   - `kustomize build config/default > modelmesh.yaml`
   - `kustomize build config/runtimes --load-restrictor LoadRestrictionsNone > modelmesh-runtimes.yaml`
   - `cp config/dependencies/quickstart.yaml modelmesh-quickstart-dependencies.yaml`

7. Generate config archive by running either one of the following commands depending
   on what version of `tar` you have. Be sure to replace the `update-me` below with
   the correct version:

   - GNU tar (Linux): `RELEASE=update-me; tar -zcvf config-${RELEASE}.tar.gz config/ --transform s/config/config-${RELEASE}/`
   - BSD tar (MacOS): `RELEASE=update-me; tar -zcvf config-${RELEASE}.tar.gz -s /config/${RELEASE}/ config/`

8. Once everything has settled, tag and push the release with `git tag $VERSION`
   and `git push upstream $VERSION`. You can also tag the release in the GitHub UI.
   The `modelmesh-controller` image will be published via GitHub Actions.

9. Create the new release in the GitHub UI and upload the generated install manifests
   to GitHub release assets: https://github.com/kserve/modelmesh-serving/releases/new
