from flask import Flask, jsonify
import random

app = Flask(__name__)

class NodeValue:
    def __init__(self, worker, value):
        self.worker = worker
        self.value = value

@app.route('/', methods=['GET'])
def health():
    return "Hello, World, I'm alive!"


@app.route('/inference/<token>', methods=['GET'])
def get_inference(token):
    random_float = str(random.uniform(0.0, 100.0))
    return random_float


# Multi-label (e.g. classification) inference. Returns a JSON array of
# {"label", "value"} objects whose values form a probability distribution
# over the labels (they sum to 1.0).
@app.route('/labeled-inference/<token>', methods=['GET'])
def get_labeled_inference(token):
    labels = ["UP", "MID", "DOWN"]
    weights = [random.random() for _ in labels]
    total = sum(weights) or 1.0
    return jsonify([
        {"label": label, "value": str(weight / total)}
        for label, weight in zip(labels, weights)
    ])


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


# Multi-label ground truth. Returns a JSON array of {"label", "value"} objects,
# here a one-hot vector marking the realized class.
@app.route('/labeled-truth/<token>/<blockheight>', methods=['GET'])
def get_labeled_truth(token, blockheight):
    labels = ["UP", "MID", "DOWN"]
    winner = random.randrange(len(labels))
    return jsonify([
        {"label": label, "value": "1.0" if i == winner else "0.0"}
        for i, label in enumerate(labels)
    ])


@app.route('/is_never_negative', methods=['POST'])
def is_never_negative():
    return jsonify({"is_never_negative":True})


@app.route('/calculate', methods=['POST'])
def calculate_loss():
    return jsonify({"loss":"1.0"})


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000)
