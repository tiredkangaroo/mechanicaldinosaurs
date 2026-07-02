from django.contrib.auth.decorators import login_required
from django.shortcuts import render, HttpResponse, redirect, reverse
from os import environ

automation_engine_url = environ.get("AUTOMATION_ENGINE_URL")
automation_engine_secret = environ.get("AUTOMATION_ENGINE_SECRET")

def automations(request):
    # get automations from automation engine
    automations = []
    try:
        response = requests.get(f"{automation_engine_url}/api/automations", headers={"Authorization": f"Bearer {automation_engine_secret}"})
        if response.status_code == 200:
            automations = response.json()
        else:
            return HttpResponse(f"error fetching automations: {response.status_code} - {response.text}", status=response.status_code)
    except Exception as e:
        return HttpResponse(f"error fetching automations: {e}", status=500)
    
    return render(request, "automations.html", {"automations": automations})
