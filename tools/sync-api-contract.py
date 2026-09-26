#!/usr/bin/env python3
"""Generate MCP input schemas from the existing integration API. No new API contract."""
import argparse, json
from pathlib import Path
import yaml
root=Path(__file__).resolve().parents[1]
parser=argparse.ArgumentParser()
parser.add_argument('--check',action='store_true')
args=parser.parse_args()
source=yaml.safe_load((root.parent/'myscoutee-client/openapi.yaml').read_text())
allowed={'event','encounters','inviteParticipants','inviteGroupParticipants','createAssets','createEvents'}
def resolve(value):
 if isinstance(value,list): return [resolve(x) for x in value]
 if not isinstance(value,dict): return value
 if '$ref' in value:
  node=source
  for part in value['$ref'][2:].split('/'): node=node[part]
  return resolve(node)
 result={k:resolve(v) for k,v in value.items() if k not in ('example','nullable','discriminator')}
 if value.get('nullable') and isinstance(result.get('type'),str): result['type']=[result['type'],'null']
 if result.get('type')=='object': result.setdefault('additionalProperties',False)
 return result
operations=[]
for path,methods in source['paths'].items():
 for method,op in methods.items():
  if not isinstance(op,dict) or op.get('operationId') not in allowed: continue
  schema={'type':'object','properties':{},'required':[],'additionalProperties':False}
  params=[]
  for param in op.get('parameters',[]):
   schema['properties'][param['name']]=resolve(param['schema'])
   if param.get('required'): schema['required'].append(param['name'])
   params.append({'name':param['name'],'in':param['in']})
  body=op.get('requestBody',{}).get('content',{}).get('application/json',{}).get('schema')
  if body:
   schema['properties']['request']=resolve(body)
   schema['required'].append('request')
  operations.append({'name':op['operationId'],'method':method.upper(),'path':path,'description':op['summary'],'schema':schema,'parameters':params})
assert {op['name'] for op in operations}==allowed
out=root/'internal/api/contract.json'
content=json.dumps(operations,indent=2,ensure_ascii=False)+'\n'
if args.check:
 assert out.read_text()==content,'Run tools/sync-api-contract.py'
else: out.write_text(content)
print('PASS: six MCP API schemas match myscoutee-client/openapi.yaml')
