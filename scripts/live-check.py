#!/usr/bin/env python3
"""Exercise real testnet payments, export evidence, assert observed settlements."""
import argparse,json,os,time,urllib.request,urllib.error
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--base',default='http://127.0.0.1:8098');p.add_argument('--out',default='docs/evidence');p.add_argument('--scenario',choices=['all','before_submit','lost_confirmation','delayed_lookup'],default='all');a=p.parse_args()
headers={'Content-Type':'application/json','User-Agent':'LedgerDrill-LiveCheck/0.1'}
if os.environ.get('LEDGERDRILL_TOKEN'):headers['Authorization']='Bearer '+os.environ['LEDGERDRILL_TOKEN']
def request(path,body=None):
    req=urllib.request.Request(a.base+path,data=json.dumps(body).encode() if body is not None else None,headers=headers)
    with urllib.request.urlopen(req,timeout=100) as r:return json.load(r)
scenarios=['before_submit','lost_confirmation','delayed_lookup'] if a.scenario=='all' else [a.scenario]
out=Path(a.out);out.mkdir(parents=True,exist_ok=True)
for scenario in scenarios:
    run=request('/api/runs',{'scenario':scenario});print(scenario,run['id'],flush=True)
    deadline=time.monotonic()+380
    while run['status'] in ('funding','running'):
        if time.monotonic()>deadline:raise RuntimeError('Timed out waiting for run; inspect its saved report')
        time.sleep(3);run=request('/api/runs/'+run['id'])
    (out/(scenario+'-'+run['id']+'.json')).write_text(json.dumps(run,indent=2)+'\n')
    if run['status']!='complete':raise RuntimeError(json.dumps(run,indent=2))
    expected=[1,1] if scenario=='before_submit' else [2,1]
    observed=[c['settlements'] for c in run['clients']]
    assert observed==expected,(scenario,observed,expected)
    for c,count in zip(run['clients'],expected):
        assert float(c['received'])==count
        assert len({t['hash'] for t in c['transactions'] if t['confirmed'] and t['successful']})==count
    print('verified settlements:',observed,flush=True)
print('Live testnet evidence written to',out)
