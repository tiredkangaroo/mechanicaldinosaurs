import socket
from os import environ

proxy_disconnect_host = environ.get("PROXY_DISCONNECT_HOST")
proxy_disconnect_port = environ.get("PROXY_DISCONNECT_PORT")
proxy_disconnect_secret = environ.get("PROXY_DISCONNECT_SECRET")

def disconnect_session(session_id):
    if len(proxy_disconnect_secret) != 128:
        print("error: disconnect secret is not 128 bytes.")
        return
    if len(session_id) != 36:
        print("error: session id is not 36 bytes.")
        return

    try:
        print(f"Connecting to disconnect service at {proxy_disconnect_host}:{proxy_disconnect_port}...")
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(5.0) # 5 second timeout should be enough i think
            s.connect((proxy_disconnect_host, int(proxy_disconnect_port)))
            
            s.sendall(proxy_disconnect_secret.encode('utf-8'))
            s.sendall(session_id.encode('utf-8'))
            
            print(f"successfully sent disconnect signal for session: {session_id}")
    except socket.timeout:
        print("timeout: failed to connect to disconnect service")
    except ConnectionRefusedError:
        print("connection refused: disconnect service is not running??")
    except Exception as e:
        print(f"unexpected error occurred: {e}")