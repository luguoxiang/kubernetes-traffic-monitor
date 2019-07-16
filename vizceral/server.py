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

PROMETHEUS_HOST = os.getenv("VIZ_PROMETHEUS_HOST", "traffic-prometheus")
PROMETHEUS_PORT = os.getenv("VIZ_PROMETHEUS_PORT", "9090")
PROMETHEUS_METRIC = "requests_total"

app = Flask(__name__, static_url_path='/static')

logging.basicConfig(level=logging.INFO)

LOGGER = logging.getLogger("traffic-monitor")


def get_metric(metric, value):
    result = {}
    result['src'] = "%s.%s" % (metric['source'], metric['source_ns']) if metric['source'] else "UNKNOWN"
    result['dst'] = "%s.%s" % (metric['destination'], metric['destination_ns'])
    result['response_code'] = int(metric['response_code'])
    result['destination_port'] = metric['destination_port']
    result['count'] = int(value[1])
    return result

def build_vizceral_metrics(status):
    metric = {}
    for code, group in status:
        if 200 <= code < 400:
            key = 'normal'
        elif 400 <= code < 500:
            key = 'danger'
        else:
            key = 'warning'      
        metric[key] = int(group['count'].sum())
    return metric

def build_vizceral_connections(connections):
    return [ {
                 "source": key[0],
                 "target": key[1],
                "metrics": build_vizceral_metrics(group.groupby(['response_code'])),
                "class": "normal",
                "metadata": {'nodeType': 'deployment'},
         } for key, group in connections ]

    
def build_vizceral_nodes(nodes):
    return [{
                "name": node,
                "displayName": node,
                "class": "normal",
                "metadata": {},
                "renderer": "focusedChild"
    } for node in nodes ]

    
def build_vizceral_graph(result):
    if 'data' not in result or 'result' not in result["data"]:
        return {}
    
    data = pd.DataFrame([ get_metric(item['metric'], item['value']) for item in result["data"]['result']])
    
    nodes = set(data['src'].unique()) | set(data['dst'].unique())
    connections = data.groupby(['src','dst'])
    
    return {"renderer": "region",
             "name": "graph",
             "serverUpdateTime":int(round(time.time() * 1000)),
             "nodes": build_vizceral_nodes(nodes),
             "connections": build_vizceral_connections(connections)}

    
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
