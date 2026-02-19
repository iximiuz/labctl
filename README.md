# iximiuz Labs control - Start Remote microVM Playgrounds From The Command Line

This is a command-line client for [iximiuz Labs](https://labs.iximiuz.com).
You can use it to start and access Linux, Docker, Kubernetes, networking, and other types of DevOps playgrounds.
Playgrounds are ephemeral, disposable, and secure enough for most learning, experimentation, and research use cases.

Some popular playgrounds:

- [ubuntu](https://labs.iximiuz.com/playgrounds/ubuntu) - A vanilla Ubuntu server.
- [k3s](https://labs.iximiuz.com/playgrounds/k3s) - A multi-node K3s cluster with a load balancer, Helm, and more.
- [docker](https://labs.iximiuz.com/playgrounds/docker) - A Linux server with Docker engine pre-installed.
- [podman](https://labs.iximiuz.com/playgrounds/podman) - A Linux server with Podman, a daemonless Docker alternative.
- [mini-lan-ubuntu](https://labs.iximiuz.com/playgrounds/mini-lan-ubuntu) - Four refined Ubuntu VMs connected into a single network.
- [k8s-client-go](https://labs.iximiuz.com/playgrounds/k8s-client-go) - Mini-programs demonstrating Kubernetes client-go usage.
- [golang](https://labs.iximiuz.com/playgrounds/golang) - A fresh Go version and a loaded VS Code (or Vim) is all you need.

See the full list of playgrounds at [labs.iximiuz.com/playgrounds](https://labs.iximiuz.com/playgrounds).

## 🎬 Getting started

Check out this short recording on YouTube to get started:

<div align="center">
  <a target="_blank" href="https://youtu.be/7JOY9YpF8f0"><img src="https://img.youtube.com/vi/7JOY9YpF8f0/0.jpg" alt="Getting started with labctl"></a>
</div>

## Installation

The command below will download the latest release to `~/.iximiuz/labctl/bin`, adding it to your PATH.

```sh
curl -sf https://labs.iximiuz.com/cli/install.sh | sh
```

`labctl` is also available via Homebrew on macOS and Linux:

```sh
brew install labctl
```

## Usage

### Authentication

First, you need to authenticate the CLI session with iximiuz Labs.
The command below will open a browser page with a one-time use URL.

```sh
labctl auth login
```

### Starting playgrounds

Once you have authenticated, you can start a new playground with a simple:

```sh
labctl playground start docker
```

You can also automatically **open the playground in a browser** with:

```sh
labctl playground start k3s --open
```

...or **SSH into the playground's machine** with:

```sh
labctl playground start ubuntu-24-04 --ssh
```

### SSH into a playground

Once you have started a playground, you can access it with:

```sh
labctl ssh <playground-id>
```

...or run a one-off command with:

```sh
labctl ssh <playground-id> -- ls -la /
```

### Using IDE (VSCode, JetBrains, etc) to access playgrounds

You can start a playground and open it in your IDE with:

```sh
labctl playground start docker --ide cursor
```

You can use the **SSH proxy mode** to access playgrounds from your IDE:

```sh
labctl ssh-proxy <playground-id>
```

Example output:

```text
SSH proxy is running on 58279

# Connect from the terminal:
ssh -i ~/.ssh/iximiuz_labs_user ssh://laborant@127.0.0.1:58279

# For better experience, add the following to your ~/.ssh/config:
Host localhost 127.0.0.1 ::1
  IdentityFile ~/.ssh/iximiuz_labs_user
  AddKeysToAgent yes
  # UseKeychain yes # macOS only
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null

# To access the playground in Visual Studio Code:
code --folder-uri vscode-remote://ssh-remote+laborant@127.0.0.1:58279/home/laborant

# To access the playground in Cursor:
cursor --folder-uri vscode-remote://ssh-remote+laborant@127.0.0.1:58279/home/laborant


Press Ctrl+C to stop
```

After adding the above piece to your SSH config,
you'll be able to develop right on the playground machine using the [Visual Studio Code Remote - SSH extension](https://code.visualstudio.com/docs/remote/ssh) or its JetBrains counterpart.
Check out this [short recording on YouTube](https://youtu.be/wah_yLoYk0M) demonstrating the use case.

### Sharing the playground access with a web terminal

You can share the playground access with others by sending them a URL to an exposed web terminal session:

```sh
labctl expose shell <playground-id> --public
```

### Exposing HTTP(s) services running in the playground

You can expose HTTP(s) services running in the playground to the public internet with:

```sh
labctl expose port <playground-id> <port>
```

Example:

```sh
# Start a new Docker playground
PLAYGROUND_ID=$(labctl playground start -q docker)

# Run a container that listens on port 8080
labctl ssh $PLAYGROUND_ID -- docker run -p 8080:80 -d nginx:alpine

# Expose port 8080 to the internet
labctl expose port $PLAYGROUND_ID 8080 --open
```

The `labctl expose port` command supports a number of options to enable/disable HTTPS,
set the Host header and path overrides, and control the URL access.

### Port forwarding

You can securely expose any service (HTTP, TCP, UDP, etc) running in the playground to your local machine with:

```sh
labctl port-forward <playground-id> -L <local-port>:<remote-port>
```

You can also expose locally running services to the playground using **remote port forwarding** (via SSH):

```sh
labctl ssh-proxy --address <local-proxy-address> <playground-id>

ssh -i ~/.ssh/iximiuz_labs_user \
  -R <remote-host>:<remote-port>:<local-host>:<local-port> \
  ssh://root@<local-proxy-address>
```

### Listing, stopping, restarting, and destroying playgrounds

You can list recent playgrounds with:

```sh
labctl playground list
```

And stop a running playground with:

```sh
labctl playground stop <playground-id>
```

Stopping a playground shuts down its virtual machines, preserving the playground state and the VM disks in a remote storage.
You can restart a stopped playground later on using the following command:

```sh
labctl playground restart <playground-id>
```

To dispose of a running or stopped playground, completely erasing its data, use the `labctl destroy` command:

```sh
labctl playground destroy <playground-id>
```

### Signing out and deleting the CLI

You can sign out and delete the CLI session with:

```sh
labctl auth logout
```

To uninstall the CLI, just remove the `~/.iximiuz/labctl` directory.

## License

APACHE-2.0
