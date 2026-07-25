# mechanical dinosaurs!

mechanical dinosaurs manages my infra! this project is split up into a few components.

# hi reviewer!!

#### video demo

it's [here](https://bucket.mechanicaldinosaurs.net/md%20demo.mov)!

#### web demo + account

you can also check out the demo [here](https://mechanicaldinosaurs.net). to sign in, set the username to the program you're reviewing for (lowercase), and the password to "demo1234".

if you want to test out the vm display output with the demo, the demo has a tcp tunnel located at vm-proxy.mechanicaldinosaurs.net. use `cloudflared access tcp --hostname vm-proxy.mechanicaldinosaurs.net --url localhost:3831` and use localhost:3831 as the VNC/SPICE server. you can't access it directly because the vm-proxy tunnel wraps the underlying tcp connection in websockets.

# components of mechanical dinosaurs

there's a few components of this project that each manage different things.

## daemon

this daemon is intended to run on linux machines. here's what it does:

- provides info about the machine (see what it provides [here](https://github.com/tiredkangaroo/mechanicaldinosaurs/blob/3fd2d6fc3f704e7758d4dc93d2eb240584d91a5c/server/server.go#L13-L38))
- provides info on a machine's ports and the services running on those ports
- provides info on vms; allows creation, updating, deletion and state mgmt of vms. also proxies VM display output.
- provides info on what cloudflare tunnels are set up to run locally

it provides this info by running a server in the background and collecting this information on demand.

## console

the UI for the whole thing. you can manage multiple machines from the console. you can:

- view a machine, it's stats and the virtual machines running on it
- make & manage virtual machines
- make & manage kubernutes deployments
- view a machine's ports, the services related to the ports (deployments, native processes, etc.) and the cloudflare tunnel ingress to these ports (if any)
- set up and manage automations from the automation engine

## automation-engine

optional! does automations. the console must be able to connect to the automation engine. if you intend to use the `machines info data refresh` trigger, the automation engine must be able to reach the daemons.

the three-steps to an automation:

#### step 1. trigger: time, interval, machine info data refresh

#### step 2: condition: ==, >, <, >=, <=, contains, regex match (negation available)

#### step 3: action: send email (slack coming soon)

on a trigger, a condition (optional) is checked and if it returns true, the automation engine will perform the action. there is a context available to check in conditions.

the machine info data refresh trigger is a trigger that triggers whenever the automation engine has refreshed the machine info (see the type [here](https://github.com/tiredkangaroo/mechanicaldinosaurs/blob/3fd2d6fc3f704e7758d4dc93d2eb240584d91a5c/server/server.go#L13-L38)). the automation engine automatically refreshes the info on all registered machines at a set interval.

conditions have context, so you can, for example, make an automation with trigger `machine info data refresh` that has a conditional that checks the temperature of the CPU of a certain machine, and sends an email if it's too high.

## vm-proxy

optional! if the following things are true, this is needed:

- a machine, that is running a VM, has the daemon component running.
- you intend to see the VM's display output.
- you cannot directly connect to the machine's daemon. it's on a private network that you can't access all of the time.
- you do not intend to expose the machine's daemon to a public network.

the VM proxy is a raw TCP proxy. it will sit on the same network as the machines and can be made available on public networks (either via port forwarding or cloudflare tunnels). it allows you to access the VM display output, even when you're not able to connect to the machine directly.

## honorable mention: dev-test-proxy

proxies HTTP port 8000 as HTTPS port 8080 given a local server.crt and server.key file are available. this is just bc i was too lazy to figure out nginx. this isn't a real component lol.

## honorable mention pt. 2: runner

standalone golang file that runs the console, automation engine, and vm proxy all in one go for production.

# setup!

## console & database

- run a postgres instance somewhere. the postgres server needs to be accessible to the console, automation engine (opt), and vm-proxy (opt).

- set the environment variables. see .env.example

- get the k3s-config.yaml and copy it over to the frontend folder.

- create a superuser using python manage.py createsuperuser

- move static using python manage.py collectstatic

- run the console using python manage.py startproject (when working directory is frontend/console)

## machine

### virtualization

1. make sure virtualization is enabled and install libvirt and set things up running the following commands:

```bash
sudo apt-get install libvirt-dev libvirt-clients
sudo virsh net-define /etc/libvirt/qemu/networks/default.xml
sudo virsh net-start default
sudo virsh net-autostart default
```

2. make sure cgroups is enabled (this is disabled by default on raspberry pis!!!). [here's](https://docs.k3s.io/installation/requirements?os=pi#cgroups) some information.

### k3s

if this machine will also serve as the k3s control node:

```bash
curl -sfL https://get.k3s.io | sh -
```

if it will not, then set that machine up first. grab the k3s token from `/var/lib/rancher/k3s/server/node-token` from that machine. then, on this machine, run:

```bash
curl -sfL https://get.k3s.io | K3S_URL=https://<control machine ip>:6443 K3S_TOKEN=<token> sh -
```

### run the daemon

download the latest binary [here](https://github.com/tiredkangaroo/mechanicaldinosaurs/releases/tag/latest) for the right architecture.

set up a systemd service to run the daemon:

```yaml
[Unit]
Description=mechanical dinosaurs daemon
After=network.target

[Service]
ExecStart=/path/to/daemon/binary
Restart=always
Environment=KEY1=VALUE1 # see environment variables for the daemon in .env.example in this repo
Environment=KEY2=VALUE2

[Install]
WantedBy=multi-user.target
```

you might want to consider adding User and Group settings to run it as a non-root user and add give that user perms to the libvirt group with `sudo usermod -aG libvirt your_username`. you also need to make sure the libvirt user can access the data folder with rwx.

write this (you will need root!) to `/etc/systemd/system/md.service`

use systemctl to enable the service
`sudo systemctl enable md.service --now`

## automation engine

download the latest binary [here](https://github.com/tiredkangaroo/mechanicaldinosaurs/releases/tag/latest) for the right architecture.

set up a systemd service to run the daemon:

```yaml
[Unit]
Description=md automation engine
After=network.target

[Service]
ExecStart=/path/to/automation_engine/binary
Restart=always
Environment=KEY1=VALUE1 # see environment variables for the daemon in .env.example in this repo
Environment=KEY2=VALUE2

[Install]
WantedBy=multi-user.target
```

write this (you will need root!) to `/etc/systemd/system/md-automation-engine.service`

use systemctl to enable the service
`sudo systemctl enable md-automation-engine.service --now`

## vm proxy

download the latest binary [here](https://github.com/tiredkangaroo/mechanicaldinosaurs/releases/tag/latest) for the right architecture.

set up a systemd service to run the daemon:

```yaml
[Unit]
Description=vm proxy
After=network.target

[Service]
ExecStart=/path/to/vm_proxy/binary
Restart=always
Environment=KEY1=VALUE1 # see environment variables for the daemon in .env.example in this repo
Environment=KEY2=VALUE2

[Install]
WantedBy=multi-user.target
```

write this (you will need root!) to `/etc/systemd/system/md-vm-proxy.service`

use systemctl to enable the service
`sudo systemctl enable md-vm-proxy.service --now`
