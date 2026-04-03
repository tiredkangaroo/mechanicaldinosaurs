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
