from django.shortcuts import render
from django.contrib.auth.decorators import login_required
from django.core.signing import TimestampSigner

@login_required
def terminal_view(request): 
    return render(request, 'terminal.html', context)