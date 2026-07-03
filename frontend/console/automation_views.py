from django.contrib.auth.decorators import login_required
from django.shortcuts import render, HttpResponse, redirect, reverse
from os import environ
import uuid, requests, json
from .models import Machine
from django.utils.safestring import mark_safe


automation_engine_url = environ.get("AUTOMATION_ENGINE_URL")
automation_engine_secret = environ.get("AUTOMATION_ENGINE_SECRET")

def automation_detail(request, automation_id):
    # get automations from automation engine
    automation = []
    try:
        response = requests.get(f"{automation_engine_url}/api/automations/" + automation_id, headers={"Authorization": f"Bearer {automation_engine_secret}"})
        if response.status_code == 200:
            automation = response.json()
        else:
            return HttpResponse(f"error fetching automation: {response.status_code} - {response.text}", status=response.status_code)
    except Exception as e:
        return HttpResponse(f"error fetching automation: {e}", status=500)
    
    return render(request, "automation_detail.html", {"automation": automation})

def create_automation(request):
    if request.method == "POST":
        # trigger
        trigger = {
            "type": request.POST.get("trigger_type"), # values: "time", "interval", "machines info refresh"
        }
        if trigger["type"] == "time":
            trigger["time"] = request.POST.get("trigger_time")
        elif trigger["type"] == "interval":
            trigger["every"] = request.POST.get("trigger_interval")
        
        # condition
        condition = {
            "variable": request.POST.get("condition_variable_name"),
            "op": request.POST.get("condition_operator"),
            "not": request.POST.get("condition_negate") == "on",
        }
        cond_value_type = request.POST.get("condition_value_type")
        if cond_value_type == "number":
            condition["value"] = float(request.POST.get("condition_value"))
        elif cond_value_type == "boolean":
            condition["value"] = request.POST.get("condition_value").lower() == "true"
        if not request.POST.get("condition_value"):
            condition = None # no condition

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