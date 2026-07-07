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

- add updating vms
- make the ui actually look good
- secure connections (using custom certs?) from machines
- add dns & cloudflare tunnel stuff
- better statuses (critical vs. starting up)
- https://hackclub.slack.com/archives/C09B6AD4TFH/p1783354065045169

setting up the daemon?

- make sure virtualization is enabled
- install libvirt and stuff (`sudo apt-get install libvirt-dev libvirt-clients`)
- if this daemon is the master control plane? install k3s: `curl -sfL https://get.k3s.io | sh -`
- if the daemon is running on raspi (or if cgroups is disabled for any reason), you have to enable cgroups by adding cgroup_memory=1 cgroup_enable=memory to /boot/firmware/cmdline.txt (https://docs.k3s.io/installation/requirements?os=pi)
- copy over /etc/rancher/k3s/k3s.yaml to frontend

## current env vars for each app:

### console

AUTOMATION_ENGINE_URL
AUTOMATION_ENGINE_SECRET
VM_PROXY_DISCONNECT_HOST
VM_PROXY_DISCONNECT_PORT
VM_PROXY_DISCONNECT_SECRET

### daemon

MECHANICAL_DINOSAURS_DATA
API_SECRET
PORT

### vm proxy

PORT
DISCONNECT_PORT
DISCONNECT_SECRET
DB_FILE

### automation engine

AUTOMATION_ENGINE_PORT
AUTOMATION_ENGINE_SECRET
AUTOMATIONS_SAVE_PATH
RESEND_API_KEY
EMAIL_DOMAIN
