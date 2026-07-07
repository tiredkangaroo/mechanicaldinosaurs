from django.shortcuts import render, HttpResponse, redirect, reverse
from os import environ, path
from .models import Machine, Deployment
import requests
from . import k3

# needs tunnel:read and tunnel:edit
cloudflare_account_id = environ.get("CLOUDFLARE_ACCOUNT_ID")
cloudflare_api_token = environ.get("CLOUDFLARE_API_TOKEN")

# note: more of these api calls need to be wrapped in try/except
def tunnels(request):
    # this should connect the externally routed tunnel to the internal service (k3s deployment)

    tunnel_ids = []
    try:
        resp = requests.get("https://api.cloudflare.com/client/v4/accounts/{cloudflare_account_id}/tunnels", headers={
            "Authorization": f"Bearer {cloudflare_api_token}",
            "Content-Type": "application/json"
        })
        data = resp.json()
        if resp.status_code != 200:
            return HttpResponse(f"error fetching tunnels: {data.errors}", status=500)
        if not data.get("success"):
            return HttpResponse(f"error fetching tunnels: {data.errors}", status=500)
        tunnel_ids = [tunnel["id"] for tunnel in data.get("result", [])]
    except Exception as e:
        return HttpResponse(f"error fetching tunnels: {str(e)}", status=500)
    
    tunnel_tokens = {} # tunnel token -> tunnel id
    tunnel_ingress = {} # tunnel id -> list of ingress rules
    for tunnel_id in tunnel_ids:
        resp = requests.get("https://api.cloudflare.com/client/v4/accounts/{cloudflare_account_id}/cfd_tunnel/{tunnel_id}/configurations", headers={
            "Authorization": f"Bearer {cloudflare_api_token}",
            "Content-Type": "application/json"
        })
        data = resp.json()
        if resp.status_code != 200:
            return HttpResponse(f"error fetching tunnel configuration for {tunnel_id}: {data.errors}", status=500)
        if not data.get("success"):
            return HttpResponse(f"error fetching tunnel configuration for {tunnel_id}: {data.errors}", status=500)
        result = data.get("result", {})
        tunnel_ingress[tunnel_id] = result.get("ingress", [])

        resp = requests.get("https://api.cloudflare.com/client/v4/accounts/{cloudflare_account_id}/cfd_tunnel/{tunnel_id}/token", headers={
            "Authorization": f"Bearer {cloudflare_api_token}",
            "Content-Type": "application/json"
        })
        data = resp.json()
        if resp.status_code != 200:
            return HttpResponse(f"error fetching tunnel token for {tunnel_id}: {data.errors}", status=500)
        if not data.get("success"):
            return HttpResponse(f"error fetching tunnel token for {tunnel_id}: {data.errors}", status=500)
        result = data.get("result", "")
        tunnel_tokens[result] = tunnel_id

    machines = Machine.objects.all()
    machine_tunnels = {} # machine name -> list of tunnel ids
    for machine in machines:
        # get a machine's tunnel ids/tokens
        resp = requests.get(f"http://{machine.hostport}/api/tunnels/ids-and-tokens").json()
        for idOrToken in resp:
            # get the tunnel id (if it's a token, we need to look it up in the tunnel_tokens dict)
            tunnel_id = None
            if idOrToken["type"] == "id":
                tunnel_id = idOrToken["value"]
            elif idOrToken["type"] == "token":
                tunnel_id = tunnel_tokens.get(idOrToken["value"])

            if not tunnel_id:
                # note: potentially sensitive info being logged
                print(f"could not find tunnel id for machine {machine.name} with id/token {idOrToken}")
                continue
            
            if machine.name not in machine_tunnels:
                machine_tunnels[machine.name] = []
            machine_tunnels[machine.name].append(tunnel_id)
    
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

        resp = requests.get(f"http://{machine.hostport}/api/ports-services")
        if resp.status_code != 200:
            return HttpResponse(f"error fetching ports-services for machine {machine.name}: {resp.text}", status=500)
        else:
            port_service_map = resp.json() # map of port -> service name
        

        # see if this machine is the k3 machine (and also let these ports override)
        host, port = machine.hostport.split(":")
        if host == k3.kube_core_api.api_client.configuration.host: # this is the k3 control node
            for svc in kube_core_api.list_service_for_all_namespaces(watch=False).items:
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
                        "k3_service_name": svc.metadata.name,
                        "ingress_rule": None, # filled in later
                    }
                    # there's a node port and the port from service spec which are different but both are valid ways to access the service
                    # note: research why that is later and what the difference is
                    port_service_map[svc_port.node_port] = obj
                    port_service_map[svc_port.port] = obj

        # fill in ingress rule for all services if they exist
        ingress_rules = machine_tunnel_ingress.get(machine.name, [])
        for ingress_rule in ingress_rules:
            service = ingress_rule.get("service")
            port = urlparse(service).port
            if port in port_service_map:
                port_service_map[port]["ingress_rule"] = ingress_rule   

        
        
        machine_exposed_port_services[machine.name] = port_service_map

    print(f"machine_exposed_port_services: {machine_exposed_port_services}")
    return render(request, "tunnels.html", {
        "machine_exposed_port_services": machine_exposed_port_services,
    })    


# def get_all_zones():
#     zones = []
#     page = 1
#     while True:
#         url = f"https://api.cloudflare.com/client/v4/zones?page={page}&per_page=50"
#         data = requests.get(url, headers={
#             "Authorization": f"Bearer {cloudflare_api_token}",
#             "Content-Type": "application/json"
#         }).json()
#         if not data or not data.get("success"):
#             break
        
#         for zone in data.get("result", []):
#             zones.append({"id": zone["id"], "name": zone["name"]})

#         # paginated api            
#         result_info = data.get("result_info", {})
#         if page >= result_info.get("total_pages", 1):
#             break
#         page += 1
#     return zones

# def get_tunnel_records(zone_id, zone_name):
#     records = []
#     page = 1
#     while True:
#         # they appear as cname records
#         url = f"https://api.cloudflare.com/client/v4/zones/{zone_id}/dns_records?type=CNAME&page={page}&per_page=100"
#         data = requests.get(url, headers={
#             "Authorization": f"Bearer {cloudflare_api_token}",
#             "Content-Type": "application/json"
#         }).json()
#         if not data or not data.get("success"):
#             break
            
#         for record in data.get("result", []):
#             # cloudflare tunnel targets always end with .cfargotunnel.com, i hope this isn't hacky bc this could break if they change it in the future
#             if record.get("content", "").endswith(".cfargotunnel.com"):
#                 records.append({
#                     "domain": zone_name,
#                     "name": record["name"],
#                     "type": record["type"],
#                     "tunnel_target": record["content"],
#                 })
                
#         result_info = data.get("result_info", {})
#         if page >= result_info.get("total_pages", 1):
#             break
#         page += 1
#     return records

# def get_all_tunnel_records():
#     all_records = []
#     zones = get_all_zones()
#     for zone in zones:
#         zone_id = zone["id"]
#         zone_name = zone["name"]
#         records = get_tunnel_records(zone_id, zone_name)
#         if records:
#             all_records.extend(records) # what a cool way to merge lists in python, i love it
#     return all_records

# def map_tunnel_to_tunnel_records(data):
#     tunnel_map = {}
#     for record in data:
#         tunnel_target = record.get("tunnel_target")
#         if tunnel_target:
#             if tunnel_target not in tunnel_map:
#                 tunnel_map[tunnel_target] = []
#             tunnel_map[tunnel_target].append(record)
#     return tunnel_map