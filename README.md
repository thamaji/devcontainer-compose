devcontainer-compose
====

VSCode Remote Container で devcontainer の中で docker-compose を使うときの volumes の問題をなんとなく解決します。

devcontainer 内で docker を使うために `/var/run/docker.sock` をマウントしていると、volumes で指定したパスが devcontainer 内のパスではなくホストのパスとして解釈されます。

このツールは docker-compose をラップし、volumes のパスを devcontainer 内のパスとして解釈させるものです。

## Usage

環境変数 CONTAINER_WORKSPACE と LOCAL_WORKSPACE が必要です。

`.devcontainer/devcontainer.json` に以下の設定を追加してください。

```
"containerEnv": {
    "CONTAINER_WORKSPACE": "${containerWorkspaceFolder}",
    "LOCAL_WORKSPACE": "${localWorkspaceFolder}"
}
```

バイナリをコピーして、本家の docker-compose よりも優先されるように PATH を設定します。

たとえば、Dockerfile に以下のように記述します。

```
RUN set -x \
    && mkdir -p /usr/local/devcontainer-tool/bin \
    && curl -fsSL -o /usr/local/devcontainer-tool/bin/docker-compose https://github.com/thamaji/devcontainer-compose/releases/download/v1.0.0/docker-compose \
    && chmod +x /usr/local/devcontainer-tool/bin/docker-compose
ENV PATH=/usr/local/devcontainer-tool/bin:${PATH}
```
