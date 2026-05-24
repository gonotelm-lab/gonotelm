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
from sklearn.decomposition import PCA


DATA_FILE = BASE_DIR / "testdata" / "pca_dataset.csv"


def load_dataset() -> np.ndarray:
    return np.loadtxt(DATA_FILE, delimiter=",", dtype=np.float64)


def compute_sklearn_snapshot() -> dict[str, Any]:
    dataset = load_dataset()
    model = PCA(n_components=2, svd_solver="full", whiten=False)
    transformed = model.fit_transform(dataset)

    return {
        "n_components": int(model.n_components_),
        "mean": model.mean_.tolist(),
        "components": model.components_.tolist(),
        "explained_variance": model.explained_variance_.tolist(),
        "explained_variance_ratio": model.explained_variance_ratio_.tolist(),
        "singular_values": model.singular_values_.tolist(),
        "transformed": transformed.tolist(),
    }


def to_plot_space(points: np.ndarray) -> np.ndarray:
    if points.ndim != 2:
        raise ValueError("points must be a 2D array")
    if points.shape[1] >= 2:
        return points[:, :2]
    if points.shape[1] == 1:
        return np.hstack([points, np.zeros((points.shape[0], 1), dtype=points.dtype)])
    raise ValueError("points must have at least one column")


def render_comparison_plot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    vectors = np.asarray(payload["vectors"], dtype=np.float64)
    sklearn_snapshot = payload["sklearn_snapshot"]
    go_snapshot = payload["go_snapshot"]

    sklearn_transformed = to_plot_space(np.asarray(sklearn_snapshot["transformed"], dtype=np.float64))
    go_transformed = to_plot_space(np.asarray(go_snapshot["transformed"], dtype=np.float64))
    colors = np.arange(vectors.shape[0], dtype=np.int64)

    fig, axes = plt.subplots(1, 2, figsize=(12, 5), dpi=150, sharex=True, sharey=True)

    axes[0].scatter(
        sklearn_transformed[:, 0],
        sklearn_transformed[:, 1],
        c=colors,
        cmap="viridis",
        s=36,
        alpha=0.9,
        edgecolors="none",
    )
    axes[0].set_title(
        "sklearn PCA\n"
        f"EVR sum={float(np.sum(np.asarray(sklearn_snapshot['explained_variance_ratio']))):.4f}"
    )
    axes[0].set_xlabel("PC1")
    axes[0].set_ylabel("PC2")
    axes[0].grid(alpha=0.2, linewidth=0.5)

    axes[1].scatter(
        go_transformed[:, 0],
        go_transformed[:, 1],
        c=colors,
        cmap="viridis",
        s=36,
        alpha=0.9,
        edgecolors="none",
    )
    axes[1].set_title(
        "Go PCA\n"
        f"EVR sum={float(np.sum(np.asarray(go_snapshot['explained_variance_ratio']))):.4f}"
    )
    axes[1].set_xlabel("PC1")
    axes[1].set_ylabel("PC2")
    axes[1].grid(alpha=0.2, linewidth=0.5)

    fig.suptitle(str(payload.get("case_name", "pca")) + ": Go vs sklearn PCA", fontsize=12)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / "pca_compare.png")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)

    return {"output_file": str(output_path)}


class TestSklearnPCAReference(unittest.TestCase):
    def test_snapshot_is_well_formed(self) -> None:
        snapshot = compute_sklearn_snapshot()

        self.assertEqual(int(snapshot["n_components"]), 2)
        self.assertEqual(len(snapshot["mean"]), 4)
        self.assertEqual(len(snapshot["components"]), 2)
        self.assertEqual(len(snapshot["transformed"]), 12)
        self.assertTrue(np.isfinite(np.asarray(snapshot["transformed"], dtype=np.float64)).all())


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
