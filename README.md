# mechanical dinosaurs!

mechanical dinosaurs manages my infra! this project is split up into a few components.

# components of mechanical dinosaurs

there's a few components of this project that each manage different things.

## daemon

this daemon is intended to run on linux machines ("machine"). here's what it does

- provides info about the machine (see what it provides [here](https://github.com/tiredkangaroo/mechanicaldinosaurs/blob/3fd2d6fc3f704e7758d4dc93d2eb240584d91a5c/server/server.go#L13-L38))
- provides info on a machine's ports
- provides info on vms. allows creation, updating, deletion and state mgmt of vms. also proxies VM display output.
- provides info on what cloudflare tunnels are running locally

## vm-proxy

it's just a raw TCP proxy intended to proxy the VM output that is proxied by the daemon. the daemon can sit on a
private network, therefore to expose the VM output, the vm-proxy will sit as a middleman between the private and public network.

it is intended to run on the same system as the console and they share a database.

## automation-engine

does automations. must be able to connect to daemons. the console must be able to connect to the automation engine. automations are a three-step thing.

#### step 1. trigger: time, interval, machine info data refresh

#### step 2: condition: ==, >, <, >=, <=, contains, regex match (negation available)

#### step 3: action: send email (slack coming soon)

on a trigger, a condition (optional) is checked and if it returns true, the automation engine will perform the action. there is a context available to check in conditions.

the machine info data refresh trigger is a trigger that triggers whenever the automation engine has refreshed the machine info (see the type [here](https://github.com/tiredkangaroo/mechanicaldinosaurs/blob/3fd2d6fc3f704e7758d4dc93d2eb240584d91a5c/server/server.go#L13-L38)). the automation engine automatically refreshes the info on all registered machines at a set interval.

conditions have context, so you can, for example, make an automation with trigger `machine info data refresh` that has a conditional that checks the temperature of the CPU of a certain machine, and sends an email if it's too high.

## console

the UI for the whole thing. you can manage multiple machines from the console. you can:

- view a machine, it's stats and the virtual machines running on it
- make & manage virtual machines
- make & manage kubernutes deployments
- view a machine's ports, the services related to the ports (deployments, native processes, etc.) and the cloudflare tunnel ingress to these ports (if any)
- set up and manage automations from the automation engine

## honorable mention: dev-test-proxy

proxies HTTP port 8000 as HTTPS port 8080 given a local server.crt and server.key file are available. this is just bc i was too lazy to figure out nginx. this isn't a real component lol.

# set up a machine

make sure virtualization is enabled and install libvirt and set things up running the following commands:

```bash
sudo apt-get install libvirt-dev libvirt-clients
sudo virsh net-define /etc/libvirt/qemu/networks/default.xml
sudo virsh net-start default
sudo virsh net-autostart default
```
