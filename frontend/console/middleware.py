# class SubdomainMiddleware:
#     def __init__(self, get_response):
#         self.get_response = get_response

#     def __call__(self, request):
#         host = request.get_host().split(':')[0]  # remove port
#         host_parts = host.split('.')

#         request.vm_name = None
#         request.machine_name = None

#         # get subdomain from host
#         if 'localhost' in host:
#             # 'wildcard', 'vms', 'localhost'
#             if len(host_parts) >= 4 and host_parts[-3] == 'vms':
#                 request.vm_name = host_parts[0]
#                 request.machine_name = host_parts[-2]
#         else:
#             # 'name', 'vms', 'machine' 'mechanicaldinosaurs', 'net'
#             if len(host_parts) >= 5 and host_parts[-4] == 'vms':
#                 request.vm_name = host_parts[0]
#                 request.machine_name = host_parts[-3]

#         return self.get_response(request)

from .models import AuditLog
from django.contrib.auth.models import User
import json

# this is basically just a request logger :sob:
class AuditLoggerMiddleware:
    def __init__(self, get_response):
        self.get_response = get_response 

    def __call__(self, request):
        print(str(request.headers))
        detail = json.dumps({
            "headers": str(request.headers),
            "body": request.body.decode("utf-8")
        })
        u = request.user
        if not isinstance(u, User):
            u = None
        print(request.headers, request.body)
        new_log = AuditLog(action=f"{request.method} {request.path}", detail=detail, ipaddr=request.environ["REMOTE_ADDR"], user=u)
        new_log.save()
        return self.get_response(request)