## Overivew

The aim was to set ourselves up with a test system that would give us an automated way to run tests against our PRs.  At the outset of this, our repo had no tests of any sort, so the first task became getting some basic tests running against the bits in our repo.

Our tests are based on the utilities found in https://github.com/openshift/origin/tree/master/hack/lib which are a set of bash functions and scripts that facilitate a reasonably fast way to develop and run a set of tests against either OpenShift itself or, in our case, a set of applications running on OpenShift.  Those tests were adapted for use in radanalytics and then re-adapted them for testing operators running in OpenShift.  We have borrowed their test runner (our fork is [here](https://github.com/crobby/peak)) that will search a subdirectory tree for scripts that match a given regular expression (ie:  ‘tests’ would find all scripts that have ‘tests’ anywhere in their full path or name), so it is easy to run a single test or a large group of tests.

Each test script has a small amount of boilerplate code followed by a series of bash tests.  Each test could call out to another utility/language/whatever.  The utilities available in the testing library can check for specific results in text/exit code/etc of each call.  Any test lines that produce a failed result are tabulated and reported at the end of the test runs.  Of course, the stdout/stderr of each failed call is also available in addition to whatever other logging your test call might produce.  Here’s what I would call the main building block of the tests:  https://github.com/openshift/origin/blob/master/hack/lib/cmd.sh It defines what amount to wrappers around whatever calls you want to make in your tests and handles the parsing of the result text/exit codes.

## Integration with OpenShift-CI:

The first step toward integrating with [OpenShift CI](https://github.com/openshift/release) is granting access to the OpenShift CI Robot and the OpenShift Merge Robot entities in the settings of the repo.  Once that is complete, you can contact the openshift-ci team and they will set up the necessary webhooks in the target repo.

Next is the prow configuration.  The configuration files are kept in the https://github.com/openshift/release repository.  Under the `core_services/prow` directory, you’ll need to modify `_config.yaml` and `_plugins.yaml` in order to have your repository included in the configuration.  Submit a PR to that repo and when it merges, you’re all set.  As for the contents of your changes, unless you know exactly what you want, it might be useful to start by adding your repo with settings copied from another repository already in the config.

Lastly, and perhaps the most important is defining the configuration that will run your tests.  These files are also in the openshift/release repo.  To define your test job, you can create a config file like ours which is defined [here](https://github.com/openshift/release/blob/master/ci-operator/config/opendatahub-io/odh-manifests/opendatahub-io-odh-manifests-master.yaml).  The job defines the following items:  
1) An image that can be built to run your tests (your tests run inside a container) 
2) Instructions on how to build that test image and 
3) A workflow that has your test or tests in the “tests” portion of the workflow definition.  In our case, we are using the ipi-aws workflow which will spin-up a fresh OpenShift cluster in AWS where our tests will run (our test container will start with an admin KUBECONFIG for that cluster)

For greater detail on any of the steps, you can refer to the [OpenShift relase README](https://github.com/openshift/release/blob/master/README.md)