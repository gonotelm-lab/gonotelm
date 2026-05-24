from __future__ import annotations

import argparse
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from typing import Any

BASE_DIR = Path(__file__).resolve().parent
MPLCONFIGDIR = Path(tempfile.gettempdir()) / "gonotelm-mplconfig"
MPLCONFIGDIR.mkdir(parents=True, exist_ok=True)
os.environ.setdefault("MPLCONFIGDIR", str(MPLCONFIGDIR))

import matplotlib.pyplot as plt
import numpy as np
from sklearn.manifold import TSNE, trustworthiness


DATA_FILE = BASE_DIR / "testdata" / "tsne_dataset.csv"

PARAMS = {
    "n_components": 2,
    "perplexity": 5.0,
    "early_exaggeration": 12.0,
    "learning_rate": "auto",
    "max_iter": 600,
    "n_iter_without_progress": 300,
    "min_grad_norm": 1e-7,
    "init": "pca",
    "method": "exact",
    "random_state": 42,
}


def load_dataset() -> np.ndarray:
    return np.loadtxt(DATA_FILE, delimiter=",", dtype=np.float64)


def condensed_pairwise_distances(embedding: np.ndarray) -> list[float]:
    n_samples = embedding.shape[0]
    distances: list[float] = []
    for i in range(n_samples):
        for j in range(i + 1, n_samples):
            distances.append(float(np.linalg.norm(embedding[i] - embedding[j])))
    return distances


def compute_sklearn_snapshot() -> dict[str, Any]:
    dataset = load_dataset()
    model = TSNE(**PARAMS)
    embedding = model.fit_transform(dataset)
    tw = trustworthiness(dataset, embedding, n_neighbors=5, metric="euclidean")

    return {
        "params": PARAMS,
        "n_iter": int(model.n_iter_),
        "learning_rate": float(model.learning_rate_),
        "kl_divergence": float(model.kl_divergence_),
        "trustworthiness": float(tw),
        "embedding": embedding.tolist(),
        "condensed_distances": condensed_pairwise_distances(embedding),
    }


def render_comparison_plot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    sklearn_snapshot = payload["sklearn_snapshot"]
    go_snapshot = payload["go_snapshot"]

    sklearn_embedding = np.asarray(sklearn_snapshot["embedding"], dtype=np.float64)
    go_embedding = np.asarray(go_snapshot["embedding"], dtype=np.float64)
    colors = np.arange(sklearn_embedding.shape[0], dtype=np.int64)

    fig, axes = plt.subplots(1, 2, figsize=(12, 5), dpi=150, sharex=True, sharey=True)

    axes[0].scatter(
        sklearn_embedding[:, 0],
        sklearn_embedding[:, 1],
        c=colors,
        cmap="plasma",
        s=36,
        alpha=0.9,
        edgecolors="none",
    )
    axes[0].set_title(
        "sklearn t-SNE\n"
        f"KL={float(sklearn_snapshot['kl_divergence']):.4f}  "
        f"Trust={float(sklearn_snapshot['trustworthiness']):.4f}"
    )
    axes[0].set_xlabel("Dim 1")
    axes[0].set_ylabel("Dim 2")
    axes[0].grid(alpha=0.2, linewidth=0.5)

    axes[1].scatter(
        go_embedding[:, 0],
        go_embedding[:, 1],
        c=colors,
        cmap="plasma",
        s=36,
        alpha=0.9,
        edgecolors="none",
    )
    axes[1].set_title(
        "Go t-SNE\n"
        f"KL={float(go_snapshot['kl_divergence']):.4f}  "
        f"Trust={float(go_snapshot['trustworthiness']):.4f}"
    )
    axes[1].set_xlabel("Dim 1")
    axes[1].set_ylabel("Dim 2")
    axes[1].grid(alpha=0.2, linewidth=0.5)

    fig.suptitle(str(payload.get("case_name", "tsne")) + ": Go vs sklearn t-SNE", fontsize=12)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / "tsne_compare.png")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)

    return {"output_file": str(output_path)}


class TestSklearnTSNEReference(unittest.TestCase):
    def test_snapshot_is_well_formed(self) -> None:
        snapshot = compute_sklearn_snapshot()

        self.assertEqual(snapshot["params"], PARAMS)
        self.assertEqual(np.asarray(snapshot["embedding"], dtype=np.float64).shape, (18, 2))
        self.assertEqual(len(snapshot["condensed_distances"]), 153)
        self.assertTrue(np.isfinite(float(snapshot["kl_divergence"])))
        self.assertTrue(0.0 <= float(snapshot["trustworthiness"]) <= 1.0)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--snapshot-stdout",
        action="store_true",
        help="Output sklearn snapshot JSON to stdout (no intermediate file).",
    )
    parser.add_argument(
        "--plot-stdin",
        action="store_true",
        help="Read plot payload from stdin JSON and render comparison plot.",
    )
    args, remaining = parser.parse_known_args()

    if args.snapshot_stdout:
        snapshot = compute_sklearn_snapshot()
        json.dump(snapshot, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.plot_stdin:
        payload = json.load(sys.stdin)
        result = render_comparison_plot_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    unittest.main(argv=[sys.argv[0], *remaining])


if __name__ == "__main__":
    main()
