from django.contrib.auth.decorators import login_required
from django.shortcuts import render, HttpResponse, redirect, reverse
from os import environ
import uuid, requests, json
from .models import Machine
from django.utils.safestring import mark_safe
from dateutil import parser
import requests


automation_engine_url = environ.get("AUTOMATION_ENGINE_URL")
automation_engine_secret = environ.get("AUTOMATION_ENGINE_SECRET")

# note: a lot of that code was copied from the index view, maybe we can refactor it into a shared func to avoid duplication spaghetti
def automation_detail(request, automation_id):
    machines = Machine.objects.all()
    update_automation_engine(machines)

    try:
        response = requests.get(
            f"{automation_engine_url}/api/automations/{automation_id}", 
            headers={"Authorization": f"Bearer {automation_engine_secret}"}
        )
        if response.status_code != 200:
            return HttpResponse(
                f"error fetching automation: {response.status_code} - {response.text}", 
                status=response.status_code
            )
        automation = response.json()
    except Exception as e:
        return HttpResponse(f"error fetching automation: {e}", status=500)
    
    trigger = automation.get("trigger", {})
    if trigger.get("type") == "time":
        try:
            human_readable_time = parser.parse(trigger["time"]).strftime("%Y-%m-%d %H:%M:%S")
            trigger["name"] = mark_safe(f"at {human_readable_time}")
        except Exception:
            trigger["name"] = mark_safe(f"at {trigger.get('time')}")
    elif trigger.get("type") == "interval":
        trigger["name"] = mark_safe(f"every {trigger.get('every')} seconds")
    elif trigger.get("type") == "machines info refresh":
        trigger["name"] = mark_safe(f"on machine info refresh")
    
    operation_to_symbol = {
        "greater than": ">",
        "less than": "<",
        "equals": "==",
        "greater than or equal": ">=",
        "less than or equal": "<=",
    }
    condition = automation.get("condition", {})
    if condition:
        not_prefix = "NOT " if condition.get("not") else ""
        op_symbol = operation_to_symbol.get(condition.get("op"), condition.get("op", ""))
        condition["name"] = mark_safe(
            f"{not_prefix}<pre style='margin:0; padding:2px 6px;'>{condition.get('variable')} "
            f"{op_symbol} {condition.get('value')}</pre></div>"
        )

    action = automation.get("action", {})
    if action.get("type") == "email":
        action["name"] = mark_safe(f"send email to {action.get('email', {}).get('to')}")
    
    if automation.get("error_logs"):
        automation["error_logs"] = list(reversed(automation["error_logs"]))  # Show the most recent errors first
        
    return render(request, "automation_detail.html", {"automation": automation})

def enable_automation(request, automation_id):
    try:
        response = requests.post(
            f"{automation_engine_url}/api/automations/{automation_id}/enable", 
            headers={"Authorization": f"Bearer {automation_engine_secret}"}
        )
        if response.status_code != 200:
            return HttpResponse(
                f"error enabling automation: {response.status_code} - {response.text}", 
                status=response.status_code
            )
    except Exception as e:
        return HttpResponse(f"error enabling automation: {e}", status=500)
    
    return redirect(reverse("automation_detail", args=[automation_id]))

def disable_automation(request, automation_id):
    try:
        response = requests.post(
            f"{automation_engine_url}/api/automations/{automation_id}/disable", 
            headers={"Authorization": f"Bearer {automation_engine_secret}"}
        )
        if response.status_code != 200:
            return HttpResponse(
                f"error disabling automation: {response.status_code} - {response.text}", 
                status=response.status_code
            )
    except Exception as e:
        return HttpResponse(f"error disabling automation: {e}", status=500)

    return redirect(reverse("automation_detail", args=[automation_id]))

def create_automation(request):
    if request.method == "POST":
        # trigger
        trigger = {
            "type": request.POST.get("trigger_type"), # values: "time", "interval", "machines info refresh"
        }
        if trigger["type"] == "time":
            trigger["time"] = request.POST.get("trigger_time")
        elif trigger["type"] == "interval":
            trigger["every"] = float(request.POST.get("trigger_interval"))
        
        # condition
        condition = {
            "variable": request.POST.get("condition_variable_name"),
            "op": request.POST.get("condition_operator"),
            "not": request.POST.get("condition_negate") == "on",
        }
        if request.POST.get("enable_condition") == "on":
            cond_value_type = request.POST.get("condition_value_type")
            if cond_value_type == "number":
                condition["value"] = float(request.POST.get("condition_value"))
            elif cond_value_type == "boolean":
                condition["value"] = request.POST.get("condition_value").lower() == "true"
        else:
            condition = None

        # action
        action = {
            "type": request.POST.get("action_type"),
        }
        if action["type"] == "email":
            action["email"] = {
                "to": request.POST.get("action_email_to"),
                "subject": request.POST.get("action_email_subject"),
                "body": request.POST.get("action_email_body"),
            }
        
        # send to automation engine
        automation_data = {
            "id": str(uuid.uuid4()),
            "enabled": False,
            "trigger": trigger,
            "condition": condition,
            "action": action,
        }
        request_headers = {
            "Authorization": f"Bearer {automation_engine_secret}",
            "Content-Type": "application/json",
        }
        try:
            response = requests.post(f"{automation_engine_url}/api/automations", json=automation_data, headers=request_headers)
            if response.status_code == 201:
                return redirect(reverse("automation_detail", args=[automation_data["id"]]))
            else:
                return HttpResponse(f"error creating automation: {response.status_code} - {response.text}", status=response.status_code)
        except Exception as e:
            return HttpResponse(f"error creating automation: {e}", status=500)
        
        return redirect("index")

    machines = Machine.objects.all()
    machine_names = [machine.name for machine in machines]
    
    return render(request, "create_automation.html", {"machine_names": mark_safe(json.dumps(machine_names))})

def clear_automation_error_logs(request, automation_id):
    try:
        response = requests.post(
            f"{automation_engine_url}/api/automations/{automation_id}/clear-logs", 
            headers={"Authorization": f"Bearer {automation_engine_secret}"}
        )
        if response.status_code != 200:
            return HttpResponse(
                f"error clearing automation logs: {response.status_code} - {response.text}", 
                status=response.status_code
            )
    except Exception as e:
        return HttpResponse(f"error clearing automation logs: {e}", status=500)

    return redirect(reverse("automation_detail", args=[automation_id]))

def delete_automation(request, automation_id):
    try:
        response = requests.post(
            f"{automation_engine_url}/api/automations/delete", 
            headers={"Authorization": f"Bearer {automation_engine_secret}"},
            json={"id": automation_id}
        )
        if response.status_code != 200: # note: should be 204
            return HttpResponse(
                f"error deleting automation: {response.status_code} - {response.text}", 
                status=response.status_code
            )
    except Exception as e:
        return HttpResponse(f"error deleting automation: {e}", status=500)

    return redirect("index")

def update_automation_engine(machines):
    machine_data_list = []
    for machine in machines:
        machine_data = {
            "name": machine.name,
            "hostport": machine.hostport,
            "secret": machine.secret_key
        }
        machine_data_list.append(machine_data)
    requests.post(f"{automation_engine_url}/api/machines", headers={"Authorization": "Bearer " + automation_engine_secret}, json=machine_data_list)