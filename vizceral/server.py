from flask import Flask
from flask import abort
from flask_jsonpify import jsonify
from flask import Response
import time
import logging
import requests
import os
import pandas as pd

# https://stackoverflow.com/questions/27981545/suppress-insecurerequestwarning-unverified-https-request-is-being-made-in-pytho
requests.packages.urllib3.disable_warnings(requests.packages.urllib3.exceptions.InsecureRequestWarning)

PROMETHEUS_HOST = os.getenv("VIZ_PROMETHEUS_HOST", "localhost")
PROMETHEUS_PORT = os.getenv("VIZ_PROMETHEUS_PORT", "9090")
PROMETHEUS_METRIC = "request_duration_seconds_bucket"

app = Flask(__name__, static_url_path='/static')

logging.basicConfig(level=logging.INFO)

LOGGER = logging.getLogger("traffic-monitor")


def get_metric(metric, value):
    result = {}
    result['src'] = "%s.%s" % (metric['source'], metric['source_ns']) if metric['source'] else "UNKNOWN"
    result['dst'] = "%s.%s" % (metric['destination'], metric['destination_ns'])
    result['response_code'] = int(metric['response_code'])
    result['le'] = metric['le']
    result['destination_port'] = metric['destination_port']
    result['count'] = int(value[1])
    return result

BUCKETS = ['0', '0.005', '0.01', '0.025', '0.05', '0.1', '0.25', '0.5', '1', '2.5','5', '10', '+Inf']

def build_vizceral_connections(connections_grp):
    result = []
    max_volumns = []
    for key, group in connections_grp:
        group_inf = group[group['le'] == '+Inf']
        
        max_volumns.append(group_inf['count'].max())
  
        metric = {'normal': 0, 'danger': 0, 'warning': 0}
        annotations = {"source": key[0], "destination": key[1], "ports": ",".join(group_inf['destination_port'].unique())}
                
        for code, sub_group in group_inf.groupby(['response_code']):
            if 200 <= code < 400:
                name = 'normal'
            elif 400 <= code < 500:
                name = 'danger'
            else:
                name = 'warning'   
            
            value = int(sub_group['count'].sum())  
            
            metric[name] = metric[name] + value
            annotations["HTTP %d" % code] = value

        res_times = {'0': 0}
        for le, sub_group in group.groupby(['le']):
            res_times[le] = int(sub_group['count'].sum())
            


        for index in range(len(BUCKETS) - 1):
            last = BUCKETS[index]
            current = BUCKETS[index + 1]
            if last in res_times and current in res_times:
                value = res_times[current] - res_times[last]
                if value > 0:
                    annotations['%s - %ss' % (last, current)] = value
                
        result.append({
                 "source": key[0],
                 "target": key[1],
                "metrics": metric,
                "annotations" : annotations,
                "class": "normal",
                "metadata": {'nodeType': 'deployment'},
         })
   
        
    return result, int(max(max_volumns))

    
def build_vizceral_nodes(nodes):
    return [{
                "name": node,
                "displayName": node,
                "class": "normal",
                "metadata": {},
                "renderer": "region"
    } for node in nodes ]

    
def build_vizceral_graph(result):
    if 'data' not in result or 'result' not in result["data"]:
        return {}
    
    data = pd.DataFrame([ get_metric(item['metric'], item['value']) for item in result["data"]['result']])
    
    nodes = set(data['src'].unique()) | set(data['dst'].unique())

    connections, maxVolume = build_vizceral_connections(data.groupby(['src','dst']))
    
    return {"renderer": "region",
             "name": "graph",
             "maxVolume": maxVolume * 2 + 20,
             "serverUpdateTime":int(round(time.time() * 1000)),
             "nodes": build_vizceral_nodes(nodes),
             "connections": connections}

    
@app.route('/vizceral')
def vizceral():
    prometheus_api = "http://%s:%s/api/v1/query" % (PROMETHEUS_HOST, PROMETHEUS_PORT)
    payload = {'query': '(%s - (%s offset 1m) ) >= 0 or %s' % (PROMETHEUS_METRIC, PROMETHEUS_METRIC, PROMETHEUS_METRIC)}
    
    response = requests.get(prometheus_api, params=payload, timeout=10)
    result = response.json()
    return jsonify(build_vizceral_graph(result))

    
if __name__ == '__main__':
    LOGGER.info("Start server at port:8080")
    app.run(host='0.0.0.0', port=8080)
