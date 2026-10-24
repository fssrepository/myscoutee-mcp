#!/usr/bin/env python3
"""Generate MCP input schemas from the existing integration API. No new API contract."""
import argparse, json
from pathlib import Path
import yaml
parser=argparse.ArgumentParser()
parser.add_argument('--check',action='store_true')
parser.add_argument('--source',type=Path,required=True,help='Application-owned OpenAPI YAML')
parser.add_argument('--contract',type=Path,required=True,help='Application-owned selected MCP contract JSON')
args=parser.parse_args()
source=yaml.safe_load(args.source.read_text())
selected=json.loads(args.contract.read_text())
allowed={op['name'] for op in selected}
metadata={op['name']:{key:value for key,value in op.items() if key not in {'name','method','path','description','schema','parameters'}} for op in selected}
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
  operations.append({'name':op['operationId'],'method':method.upper(),'path':path,'description':op['summary'],'schema':schema,'parameters':params,**metadata[op['operationId']]})
assert {op['name'] for op in operations}==allowed
out=args.contract
content=json.dumps(operations,indent=2,ensure_ascii=False)+'\n'
if args.check:
 assert out.read_text()==content,'Regenerate the selected contract with tools/sync-api-contract.py'
else: out.write_text(content)
print(f'PASS: {len(operations)} MCP API schemas match the selected OpenAPI source')
