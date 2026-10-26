#!/usr/bin/env python3
"""Public, read-only deployment checks. Never carries authentication credentials."""
import argparse,json,time,urllib.request,urllib.error

def request(url):
    with urllib.request.urlopen(urllib.request.Request(url,headers={"Cache-Control":"no-cache"}),timeout=25) as response:
        return response.status,response.read(),response.headers

def check(api,web,revision=""):
    status,body,_=request(api+"/healthz")
    health=json.loads(body)
    if status!=200 or health.get("status")!="ok": raise ValueError("Backend is not healthy")
    if revision and health.get("revision")!=revision: raise ValueError("New backend revision is not live yet")
    try: request(api+"/api/catalogs")
    except urllib.error.HTTPError as exc:
        if exc.code!=401:raise ValueError("Unauthenticated API must return 401") from None
    else:raise ValueError("API returned catalog data without a session")
    if web:
        _,body,_=request(web)
        if b'<div id="root"' not in body:raise ValueError("Frontend application root missing")
        if revision:
            _,release,_=request(web.rstrip("/")+"/release.json?revision="+revision)
            if json.loads(release).get("revision")!=revision:raise ValueError("New frontend revision is not live yet")
    return health

def main():
    p=argparse.ArgumentParser();p.add_argument("--api",default="https://catalogo-livros-api.onrender.com");p.add_argument("--web",default="https://biblioteca.praesens.tech");p.add_argument("--revision",default="");p.add_argument("--attempts",type=int,default=1);a=p.parse_args()
    for attempt in range(a.attempts):
        try:check(a.api,a.web,a.revision);print("Public health, authentication boundary and frontend checks passed");return
        except Exception as exc:
            if attempt+1==a.attempts:raise SystemExit("Deployment checks failed: "+type(exc).__name__+" (URLs and response bodies omitted)")
            time.sleep(15)
if __name__=="__main__":main()
