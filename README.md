# still cooking 🍳

this will manage and control:

- health of pineapple (the small raspberry pi running everything)
- systemd services on pineapple (i have services that run a lot of different things)
- logs and health checks on my deployments
- all of my dns stuff & cloudflare tunnels
- r2 bucket & tiredkangaroo/storage instance (on digital ocean)
- calling scripts when a repo is pushed to (such as updating the deployment on pineapple)

it'll have a concept of projects:

- a project will contain of a lot of the things above that keep it running:
- so if a project has a service, a subdomain, logs, and a deployment to run health checks on, that will be shown inside the project

it's meant to keep everything infra-related one place. it will be on my infra domain mechanicaldinosaurs.net.

# what is the daemon?

the daemon runs on the actual machines (pineapple & dragonfruit). it provides the api to get info and make changes (cpu, mem, storage, services, vms, docker, etc.).

# important commands ill need

```
# also found in /usr/share/libvirt/networks/default.xml
sudo virsh net-define /etc/libvirt/qemu/networks/default.xml
sudo virsh net-start default
sudo virsh net-autostart default
```

```
sudo apt-get install libvirt-dev libvirt-clients
```

```
curl -sfL https://get.k3s.io | K3S_URL=https://<control machine ip>:6443 K3S_TOKEN=<token from /var/lib/rancher/k3s/server/node-token> sh -
```

get the windows virtio drivers here: https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso and put them in as $MECHANICAL_DINOSAURS_DATA/drivers/virtio-win.iso

to do list:

- secure connections (using custom certs?) from machines
- notification system on the ui & by email
- add notification for when x goes down, etc.
- add dns & cloudflare tunnel stuff
- shift to k3s; phase out regular ol docker containers and compose

setting up the daemon?

- make sure virtualization is enabled
- install libvirt and stuff (`sudo apt-get install libvirt-dev libvirt-clients`)
- if this daemon is the master control plane? install k3s: `curl -sfL https://get.k3s.io | sh -`
- if the daemon is running on raspi (or if cgroups is disabled for any reason), you have to enable cgroups by adding cgroup_memory=1 cgroup_enable=memory to /boot/firmware/cmdline.txt (https://docs.k3s.io/installation/requirements?os=pi)
- copy over /etc/rancher/k3s/k3s.yaml to frontend
