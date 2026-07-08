from flask import Flask, jsonify, request
import logging

app = Flask(__name__)

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("mock-source")

# Deterministic mock values so test runs are reproducible and easy to eyeball.
# The scalar (single-label) topic is what the backwards-compatibility test uses.
SCALAR_INFERENCE = 10.0
SCALAR_GROUND_TRUTH = 12.0

# Floor applied to computed losses. The offchain node takes Log10(loss) when the
# loss function is "never negative", so a loss of exactly 0 would blow up to -inf
# and produce an invalid Dec. Keeping a small positive floor keeps every bundle
# entry finite and valid.
LOSS_FLOOR = 1e-6

# Labels used by the multi-label routes. These match the label set already on the
# classification topic (topic 5, created by the chain integration test), so the
# reputer's strict predicted-vs-truth label-set check passes. Weights sum to 1.0
# to satisfy the topic's RequireUnity constraint.
LABELS = ["UP", "MID", "LOW"]


@app.route('/', methods=['GET'])
def health():
    return "Hello, World, I'm alive!"


# Single-label (scalar) inference. Returns a bare decimal string, which is what
# the apiAdapter's CalcInference expects.
@app.route('/inference/<token>', methods=['GET'])
def get_inference(token):
    log.info("GET /inference/%s -> %s", token, SCALAR_INFERENCE)
    return str(SCALAR_INFERENCE)


# Multi-label (e.g. classification) inference. Returns a JSON array of
# {"label", "value"} objects whose values form a probability distribution
# over the labels (they sum to 1.0).
@app.route('/labeled-inference/<token>', methods=['GET'])
def get_labeled_inference(token):
    weights = [0.3, 0.2, 0.5]
    log.info("GET /labeled-inference/%s", token)
    return jsonify([
        {"label": label, "value": str(weight)}
        for label, weight in zip(LABELS, weights)
    ])


@app.route('/forecast', methods=['GET'])
def get_forecast():
    node_values = [
        {"worker": "Worker1", "value": str(SCALAR_INFERENCE)},
        {"worker": "Worker2", "value": str(SCALAR_INFERENCE)},
        {"worker": "Worker3", "value": str(SCALAR_INFERENCE)},
    ]
    log.info("GET /forecast")
    return jsonify(node_values)


# Single-label ground truth. Returns a bare decimal string.
@app.route('/truth/<token>/<blockheight>', methods=['GET'])
def get_truth(token, blockheight):
    log.info("GET /truth/%s/%s -> %s", token, blockheight, SCALAR_GROUND_TRUTH)
    return str(SCALAR_GROUND_TRUTH)


# Multi-label ground truth. Returns a JSON array of {"label", "value"} objects,
# here a one-hot vector marking the realized class.
@app.route('/labeled-truth/<token>/<blockheight>', methods=['GET'])
def get_labeled_truth(token, blockheight):
    winner = 0
    log.info("GET /labeled-truth/%s/%s (winner=%s)", token, blockheight, LABELS[winner])
    return jsonify([
        {"label": label, "value": "1.0" if i == winner else "0.0"}
        for i, label in enumerate(LABELS)
    ])


@app.route('/is_never_negative', methods=['POST'])
def is_never_negative():
    # Squared-error style losses are always >= 0, so the node will Log10 them.
    return jsonify({"is_never_negative": True})


def _as_label_map(vals):
    """Normalize a labeled [{"label","value"}] array into {label: float}."""
    out = {}
    for item in vals:
        out[item["label"]] = float(item["value"])
    return out


# Loss function service. Computes a real squared-error loss from the posted
# y_true / y_pred so the reputer path exercises the full data flow (rather than
# returning a constant). Supports both the scalar wire format (bare strings) and
# the labeled format ([{"label","value"}] arrays).
@app.route('/calculate', methods=['POST'])
def calculate_loss():
    payload = request.get_json(force=True, silent=True) or {}
    y_true = payload.get("y_true")
    y_pred = payload.get("y_pred")

    try:
        if isinstance(y_true, list) and isinstance(y_pred, list):
            # Labeled: mean of per-label squared errors, joined by label.
            truth = _as_label_map(y_true)
            pred = _as_label_map(y_pred)
            labels = set(truth) & set(pred)
            if not labels:
                raise ValueError("no overlapping labels between y_true and y_pred")
            loss = sum((truth[l] - pred[l]) ** 2 for l in labels) / len(labels)
        else:
            # Scalar: squared error.
            loss = (float(y_true) - float(y_pred)) ** 2
    except (TypeError, ValueError) as e:
        log.warning("POST /calculate bad payload (%s): %s", e, payload)
        return jsonify({"error": str(e)}), 400

    loss = max(loss, LOSS_FLOOR)
    log.info("POST /calculate y_true=%s y_pred=%s -> loss=%s", y_true, y_pred, loss)
    return jsonify({"loss": str(loss)})


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000)
