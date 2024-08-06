from flask import Flask, jsonify
import random

app = Flask(__name__)

class NodeValue:
    def __init__(self, worker, value):
        self.worker = worker
        self.value = value

@app.route('/inference/<token>', methods=['GET'])
def get_inference(token):
    random_float = str(random.uniform(0.0, 100.0))
    return random_float

@app.route('/forecast', methods=['GET'])
def get_forecast():
    node_values = [
        NodeValue("Worker1", str(random.uniform(0.0, 100.0))),
        NodeValue("Worker2", str(random.uniform(0.0, 100.0))),
        NodeValue("Worker3", str(random.uniform(0.0, 100.0))),
    ]
    return jsonify([nv.__dict__ for nv in node_values])

@app.route('/truth/<token>/<blockheight>', methods=['GET'])
def get_truth(token, blockheight):
    random_float = str(random.uniform(0.0, 100.0))
    return random_float

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=8000)
