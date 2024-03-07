# iximiuz Labs control - Start Remote microVM Playgrounds From The Command Line

This is a command line tool for [iximiuz Labs](https://labs.iximiuz.com).
You can use it to start and access Linux, Docker, Kubernetes, networking, and other types of DevOps playgrounds.
Playgrounds are ephemeral, disposable, and secure enough for happy experimentation.

Some popular playgrounds:

- [ubuntu](https://labs.iximiuz.com/playgrounds/ubuntu) - A vanilla Ubuntu server.
- [k3s](https://labs.iximiuz.com/playgrounds/k3s) - A multi-node K3s cluster with a load balancer, Helm, and more.
- [docker](https://labs.iximiuz.com/playgrounds/docker) - A Linux server with Docker engine pre-installed.
- [podman](https://labs.iximiuz.com/playgrounds/podman) - A Linux server with Podman, a daemonless Docker alternative.
- [mini-lan-ubuntu](https://labs.iximiuz.com/playgrounds/mini-lan-ubuntu) - Three refined Ubuntu VMs connected into a single network.
- [k8s-client-go](https://labs.iximiuz.com/playgrounds/k8s-client-go) - Mini-programs demonstrating Kubernetes client-go usage.
- [golang](https://labs.iximiuz.com/playgrounds/golang) - A fresh Go version and a loaded VS Code (or Vim) is all you need.

See the full list of playgrounds at [labs.iximiuz.com/playgrounds](https://labs.iximiuz.com/playgrounds).

## Installation

The below command will download the latest release to `~/.iximiuz/labctl/bin` adding it to your PATH.

```sh
curl -sf https://labs.iximiuz.com/cli/install.sh | sh
```

## Usage

### Authentication

First, you need to authenticate the CLI session with iximiuz Labs.
The below command will open a browser page with a one-time use URL.

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
labctl playground start ubuntu --ssh
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

You can use the **SSH proxy mode** to access playgrounds from your IDE:

```sh
labctl ssh-proxy <playground-id>
```

Example output:

```text
SSH proxy is running on 58279

Connect with: ssh -i ~/.iximiuz/labctl/ssh/id_ed25519 ssh://root@127.0.0.1:58279

Or add the following to your ~/.ssh/config:
Host 65ea1e10f6af43783e69fe68-docker-01
  HostName 127.0.0.1
  Port 58279
  User root
  IdentityFile ~/.iximiuz/labctl/ssh/id_ed25519
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null

Press Ctrl+C to stop
```

After adding the above piece to your SSH config,
you'll be able to develop right on the playground machine using the [Visual Studio Code Remote - SSH extension](https://code.visualstudio.com/docs/remote/ssh) or its JetBrains' counterpart.

### Port forwarding

You can securely expose any service (HTTP, TCP, UDP, etc) running in the playground to your local machine with:

```sh
labctl port-forward <playground-id> -L <local-port>:<remote-port>
```

You can also expose locally running services to the playground using **remote port forwarding** (via SSH):

```sh
labctl ssh-proxy --address <local-proxy-address> <playground-id>

ssh -i ~/.iximiuz/labctl/ssh/id_ed25519 \
  -R <remote-host>:<remote-port>:<local-host>:<local-port> \
  ssh://root@<local-proxy-address>
```

### Listing and stopping playgrounds

You can list recent playgrounds with:

```sh
labctl playground list
```

And stop a running playground with:

```sh
labctl playground stop <playground-id>
```

### Signing out and deleting the CLI

You can sign out and delete the CLI session with:

```sh
labctl auth logout
```

To uninstall the CLI, just remove the `~/.iximiuz/labctl` directory.

## License

APACHE-2.0
