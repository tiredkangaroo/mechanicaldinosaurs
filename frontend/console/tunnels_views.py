from django.shortcuts import render, HttpResponse, redirect, reverse
from os import environ, path
from .models import Machine, Deployment
import requests
from . import k3
from urllib.parse import urlparse
from django.conf import settings

# needs tunnel:read and tunnel:edit
cloudflare_account_id = settings.CLOUDFLARE_ACCOUNT_ID
cloudflare_api_token = settings.CLOUDFLARE_API_TOKEN

# note: more of these api calls need to be wrapped in try/except
def tunnels(request):
    # this should connect the externally routed tunnel to the internal service (k3s deployment)

    highlight = request.GET.get("highlight")
    print(f"highlight: {highlight}") 

    tunnel_ids = []
    try:
        resp = requests.get(f"https://api.cloudflare.com/client/v4/accounts/{cloudflare_account_id}/tunnels", headers={
            "Authorization": f"Bearer {cloudflare_api_token}",
            "Content-Type": "application/json"
        }, timeout=settings.DEFAULT_TIMEOUT)
        data = resp.json()
        if resp.status_code != 200:
            return HttpResponse(f"error fetching tunnels: {data.get('errors', ['Unknown error'])}", status=500)
        if not data.get("success"):
            return HttpResponse(f"error fetching tunnels: {data.get('errors', ['Unknown error'])}", status=500)
        tunnel_ids = [tunnel["id"] for tunnel in data.get("result", [])]
    except Exception as e:
        return HttpResponse(f"error fetching tunnels: {str(e)}", status=500)
    
    tunnel_tokens = {} # tunnel token -> tunnel id
    tunnel_ingress = {} # tunnel id -> list of ingress rules
    for tunnel_id in tunnel_ids:
        resp = requests.get(f"https://api.cloudflare.com/client/v4/accounts/{cloudflare_account_id}/cfd_tunnel/{tunnel_id}/configurations", headers={
            "Authorization": f"Bearer {cloudflare_api_token}",
            "Content-Type": "application/json"
        }, timeout=settings.DEFAULT_TIMEOUT)
        data = resp.json()
        match resp.status_code:
            case 200:
                result = data.get("result", {})
                config = result.get("config", {})
                tunnel_ingress[tunnel_id] = config.get("ingress", [])
            case 404:
                # configuration not found (not configured yet)
                pass
            case _:
                return HttpResponse(f"error fetching tunnel configuration for {tunnel_id}: {data.get('errors', ['Unknown error'])}", status=500)

        resp = requests.get(f"https://api.cloudflare.com/client/v4/accounts/{cloudflare_account_id}/cfd_tunnel/{tunnel_id}/token", headers={
            "Authorization": f"Bearer {cloudflare_api_token}",
            "Content-Type": "application/json"
        }, timeout=settings.DEFAULT_TIMEOUT)
        data = resp.json()
        match resp.status_code:
            case 200:
                result = data.get("result", "")
                tunnel_tokens[result] = tunnel_id
            case _:
                return HttpResponse(f"error fetching tunnel token for {tunnel_id}: {data.get('errors', ['Unknown error'])}", status=500)

    machines = Machine.objects.all()
    machine_tunnels = {} # machine name -> list of tunnel ids
    errors = []
    for machine in machines:
        if not machine.cf_tunnels:
            continue
        machine_tunnels[machine.name] = [tunnel.get("id") for tunnel in machine.cf_tunnels]
    
    # now we've got all the tunnels and their ingress rules, and we know which machines have which tunnels
    # which means we can associate the ingress rules with the machines

    machine_tunnel_ingress = {} # machine name -> list of ingress rules
    for machine_name, tunnel_ids in machine_tunnels.items():
        for tunnel_id in tunnel_ids:
            ingress_rules = tunnel_ingress.get(tunnel_id, [])
            if machine_name not in machine_tunnel_ingress:
                machine_tunnel_ingress[machine_name] = []
            machine_tunnel_ingress[machine_name].extend(ingress_rules)
    
    machine_exposed_port_services = {} # machine name -> {port -> {name: name, k3_deployment_name: k3_deployment_name, k3_service_name: k3_service_name, ingress_rule: ingress_rule}}
    for machine in machines:
        port_service_map = {}

        resp = None
        print("getting ports-services for machine", machine.name)
        try:
            resp = requests.get(f"http://{machine.hostport}/api/ports-services", headers={"Authorization": f"Bearer {machine.secret_key}"}, timeout=settings.DEFAULT_TIMEOUT)
        except Exception as e:
            continue
        
        if resp.status_code != 200:
            return HttpResponse(f"error fetching ports-services for machine {machine.name}: {resp.text}", status=500)
        else:
            for port, service in resp.json().items():
                port_service_map[int(port)] = {
                    "name": service,
                    "k3_deployment_name": None,
                    "k3_namespace": None,
                    "k3_service_name": None,
                    "ingress_rule": None, # filled in later
                    "highlight": service == highlight and highlight is not None, # highlight if this is the deployment we're looking for
                }
        

        # see if this machine is the k3 machine (and also let these ports override)
        machine_host, _ = machine.hostport.split(":")
        k3_host = urlparse(k3.kube_core_api.api_client.configuration.host).hostname
        if machine_host == k3_host: # this is the k3 control node
            for svc in k3.kube_core_api.list_service_for_all_namespaces(watch=False).items:
                for svc_port in svc.spec.ports:
                    k3_deployment_name = None
                    try:
                        deployment = Deployment.objects.get(service_name=svc.metadata.name)
                        k3_deployment_name = deployment.name
                    except Deployment.DoesNotExist:
                        pass # no deployment associated that we know is associated with this service
                    
                    obj = {
                        "name": svc.metadata.name,
                        "k3_deployment_name": k3_deployment_name,
                        "k3_namespace": svc.metadata.namespace,
                        "k3_service_name": svc.metadata.name,
                        "ingress_rule": None, # filled in later
                        "ingress_type": None, # also filled in later
                        "highlight": k3_deployment_name == highlight and highlight is not None, # highlight if this is the deployment we're looking for
                    }
                    # there's a node port and the port from service spec which are different but both are valid ways to access the service
                    # note: research why that is later and what the difference is
                    if svc_port.node_port:
                        port_service_map[svc_port.node_port] = obj
                    if svc_port.port:
                        port_service_map[svc_port.port] = obj
        else:
            pass

        # fill in ingress rule for all services if they exist
        ingress_rules = machine_tunnel_ingress.get(machine.name, [])
        for ingress_rule in ingress_rules:
            service = ingress_rule.get("service")
            service_parse = urlparse(service)
            
            port = service_parse.port
            if port in port_service_map:
                port_service_map[port]["ingress_rule"] = ingress_rule
                port_service_map[port]["ingress_type"] = service_parse.scheme # http, https, tcp, etc.
            elif port is not None:
                port_service_map[port] = {
                    "name": None,
                    "k3_deployment_name": None,
                    "k3_namespace": None,
                    "k3_service_name": None,
                    "ingress_rule": ingress_rule,
                    "ingress_type": service_parse.scheme,
                    "highlight": False,
                }
        
        machine_exposed_port_services[machine.name] = port_service_map
    

    # print(f"machine_exposed_port_services: {machine_exposed_port_services}")

    # sort each machine's port_service_map by port number
    for machine_name, port_service_map in machine_exposed_port_services.items():
        machine_exposed_port_services[machine_name] = dict(sorted(port_service_map.items(), key=lambda item: item[0]))

    return render(request, "tunnels.html", {
        "machine_exposed_port_services": machine_exposed_port_services,
        "machine_tunnels": machine_tunnels,
        "errors": errors,
    })    

def add_tunnel(request, machine_name):
    if request.method == "POST":
        name = request.POST.get("name")
        tunnel_id = request.POST.get("tunnel_id")
        if not tunnel_id:
            return HttpResponse("tunnel_id is required", status=400)
        
        try:
            machine = Machine.objects.get(name=machine_name)
        except Machine.DoesNotExist:
            return HttpResponse(f"machine {machine_name} does not exist", status=404)
        
        # associate the machine with this tunnel id
        tunnels = machine.cf_tunnels if machine.cf_tunnels else []
        found = False
        for tunnel in tunnels:
            if tunnel.get("id") == tunnel_id:
                found = True
                break
        if not found:
            tunnels.append({"id": tunnel_id, "name": name})
        machine.cf_tunnels = tunnels
        machine.save()

        return redirect(request.META.get("HTTP_REFERER", reverse("tunnels")))    
    else:
        return HttpResponse("method not allowed", status=405)

def remove_tunnel(request, machine_name, tunnel_id):
    if request.method == "POST":
        try:
            machine = Machine.objects.get(name=machine_name)
        except Machine.DoesNotExist:
            return HttpResponse(f"machine {machine_name} does not exist", status=404)
        
        # remove the tunnel id from the machine's cf_tunnels
        tunnels = machine.cf_tunnels if machine.cf_tunnels else []
        tunnels = [tunnel for tunnel in tunnels if tunnel.get("id") != tunnel_id]
        machine.cf_tunnels = tunnels
        machine.save()
        
        return redirect(request.META.get("HTTP_REFERER", reverse("tunnels")))
    else:
        return HttpResponse("method not allowed", status=405)