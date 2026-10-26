#!/usr/bin/env python3
"""Deploy only to configured providers; never print secret URLs or responses."""
import os,sys,re,urllib.parse,urllib.request

def configuration():
    values={k:os.environ.get(k,"") for k in ("DATABASE_URL","RENDER_DEPLOY_HOOK_URL","EXPECTED_REVISION","BACKUP_AGE_RECIPIENT")}
    missing=[k for k,v in values.items() if not v]
    if missing:raise ValueError("Missing configuration: "+", ".join(missing))
    url=urllib.parse.urlsplit(values["RENDER_DEPLOY_HOOK_URL"])
    if url.scheme!="https" or url.hostname!="api.render.com" or url.username:raise ValueError("Invalid provider URL for RENDER_DEPLOY_HOOK_URL")
    if not re.fullmatch(r"[0-9a-f]{40}",values["EXPECTED_REVISION"]):raise ValueError("A full tested commit SHA is required")
    return values

def run(mode):
    values=configuration()
    if mode=="preflight":print("Deployment configuration present");return
    if mode=="backend":
        url=urllib.parse.urlsplit(values["RENDER_DEPLOY_HOOK_URL"]);query=urllib.parse.parse_qs(url.query);query["ref"]=[values["EXPECTED_REVISION"]];target=urllib.parse.urlunsplit(url._replace(query=urllib.parse.urlencode(query,doseq=True)))
    else:raise ValueError("Unknown deployment target")
    with urllib.request.urlopen(urllib.request.Request(target,data=b"",method="POST"),timeout=30) as response:
        if response.status not in (200,201,202):raise ValueError("Provider rejected deployment")
    print(mode+" deployment requested")
if __name__=="__main__":
    try:run(sys.argv[1])
    except ValueError as exc:raise SystemExit(str(exc))
    except Exception:raise SystemExit("Deployment request failed; secret URL and provider body omitted")
