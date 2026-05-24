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
import umap
from sklearn.manifold import trustworthiness


DATA_FILE = BASE_DIR / "testdata" / "tsne_dataset.csv"

PARAMS = {
    "n_components": 2,
    "n_neighbors": 5,
    "min_dist": 0.1,
    "spread": 1.0,
    "n_epochs": 300,
    "learning_rate": 1.0,
    "negative_sample_rate": 5,
    "metric": "euclidean",
    "init": "random",
    "random_state": 42,
    "n_jobs": 1,
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


def compute_python_umap_snapshot() -> dict[str, Any]:
    dataset = load_dataset()
    model = umap.UMAP(**PARAMS)
    embedding = model.fit_transform(dataset)
    tw = trustworthiness(dataset, embedding, n_neighbors=5, metric="euclidean")

    go_style_params = dict(PARAMS)
    go_style_params.pop("n_jobs")
    go_style_params["num_workers"] = 1

    return {
        "params": go_style_params,
        "trustworthiness": float(tw),
        "embedding": embedding.tolist(),
        "condensed_distances": condensed_pairwise_distances(embedding),
    }


def render_comparison_plot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    python_snapshot = payload["python_snapshot"]
    go_snapshot = payload["go_snapshot"]

    python_embedding = np.asarray(python_snapshot["embedding"], dtype=np.float64)
    go_embedding = np.asarray(go_snapshot["embedding"], dtype=np.float64)
    colors = np.arange(python_embedding.shape[0], dtype=np.int64)

    fig, axes = plt.subplots(1, 2, figsize=(12, 5), dpi=150, sharex=True, sharey=True)

    axes[0].scatter(
        python_embedding[:, 0],
        python_embedding[:, 1],
        c=colors,
        cmap="viridis",
        s=36,
        alpha=0.9,
        edgecolors="none",
    )
    axes[0].set_title(
        "python umap-learn\n"
        f"Trust={float(python_snapshot['trustworthiness']):.4f}"
    )
    axes[0].set_xlabel("Dim 1")
    axes[0].set_ylabel("Dim 2")
    axes[0].grid(alpha=0.2, linewidth=0.5)

    axes[1].scatter(
        go_embedding[:, 0],
        go_embedding[:, 1],
        c=colors,
        cmap="viridis",
        s=36,
        alpha=0.9,
        edgecolors="none",
    )
    axes[1].set_title(
        "Go UMAP\n"
        f"Trust={float(go_snapshot['trustworthiness']):.4f}"
    )
    axes[1].set_xlabel("Dim 1")
    axes[1].set_ylabel("Dim 2")
    axes[1].grid(alpha=0.2, linewidth=0.5)

    fig.suptitle(str(payload.get("case_name", "umap")) + ": Go vs python UMAP", fontsize=12)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / "umap_compare.png")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)

    return {"output_file": str(output_path)}


class TestPythonUMAPReference(unittest.TestCase):
    def test_snapshot_is_well_formed(self) -> None:
        snapshot = compute_python_umap_snapshot()
        self.assertEqual(np.asarray(snapshot["embedding"], dtype=np.float64).shape, (18, 2))
        self.assertEqual(len(snapshot["condensed_distances"]), 153)
        self.assertTrue(0.0 <= float(snapshot["trustworthiness"]) <= 1.0)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--snapshot-stdout",
        action="store_true",
        help="Output python umap snapshot JSON to stdout (no intermediate file).",
    )
    parser.add_argument(
        "--plot-stdin",
        action="store_true",
        help="Read plot payload from stdin JSON and render comparison plot.",
    )
    args, remaining = parser.parse_known_args()

    if args.snapshot_stdout:
        snapshot = compute_python_umap_snapshot()
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
