FROM quay.io/centos/centos:stream8

# List of chromedriver versions for download - https://chromedriver.chromium.org/downloads
ARG CHROMEDRIVER_VER=104.0.5112.79
ARG ORG=opendatahub-io
ARG BRANCH=main
ARG ODS_CI_REPO=https://github.com/red-hat-data-services/ods-ci
ARG ODS_CI_GITREF=releases/1.7.0-5
ARG OC_CLI_URL=https://mirror.openshift.com/pub/openshift-v4/amd64/clients/ocp/latest/openshift-client-linux.tar.gz

ENV HOME /root
WORKDIR /root

RUN dnf install -y bc git go-toolset python3-pip unzip && \
    git clone https://github.com/crobby/peak $HOME/peak && \    
    cd $HOME/peak && \
    git submodule update --init

RUN dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm &&\
    echo -e "[google-chrome]\nname=google-chrome\nbaseurl=http://dl.google.com/linux/chrome/rpm/stable/x86_64\nenabled=1\ngpgcheck=1\ngpgkey=https://dl.google.com/linux/linux_signing_key.pub" > /etc/yum.repos.d/google-chrome.repo &&\
    dnf -y install "google-chrome-stable" &&\
    dnf clean all

# install jq to help with parsing json
RUN curl -o /usr/local/bin/jq http://stedolan.github.io/jq/download/linux64/jq && \
    chmod +x /usr/local/bin/jq

RUN mkdir -p $HOME/src && \
    cd $HOME/src && \
    git clone --depth=1 --branch main https://github.com/${ORG}/modelmesh-serving && \
    # Clone ods-ci repo at specified git ref for the JupyterHub webUI tests
    git clone --depth=1 ${ODS_CI_REPO} ods-ci && cd ods-ci && \
    git fetch origin ${ODS_CI_GITREF} && git checkout FETCH_HEAD && \
    chmod -R 777 $HOME/src

# Use a specific destination file name in case the url dow download name changes
ADD ${OC_CLI_URL} $HOME/peak/oc-cli.tar.gz
RUN tar -C /usr/local/bin -xvf $HOME/peak/oc-cli.tar.gz && \
    chmod +x /usr/local/bin/oc

RUN curl -o /tmp/chromedriver_linux64.zip -L https://chromedriver.storage.googleapis.com/${CHROMEDRIVER_VER}/chromedriver_linux64.zip &&\
    unzip /tmp/chromedriver_linux64.zip &&\
    cp chromedriver /usr/local/bin/chromedriver

COPY Pipfile Pipfile.lock $HOME/peak/

RUN pip3 install micropipenv &&\
    ln -s `which pip3` /usr/bin/pip &&\
    cd $HOME/peak &&\
    micropipenv install

COPY setup/operatorsetup scripts/install.sh scripts/installandtest.sh $HOME/peak/
COPY resources $HOME/peak/operator-tests/modelmesh/resources
COPY util $HOME/peak/operator-tests/modelmesh
COPY setup/kfctl_openshift.yaml $HOME/kfdef/
COPY basictests $HOME/peak/operator-tests/modelmesh/basictests

RUN chmod -R 777 $HOME/kfdef && \
    mkdir -p $HOME/.kube && \
    chmod -R 777 $HOME/.kube && \
    chmod -R 777 $HOME/peak && \
    mkdir -p /peak && \
    chmod -R 777 $HOME && \
    ln -s $HOME/peak/installandtest.sh /peak/installandtest.sh

# For local testing, you can add your own kubeconfig to the image
# Note:  Do not push the image to a public repo with your kubeconfig
# ADD kubeconfig /root/.kube/config

CMD $HOME/peak/installandtest.sh
