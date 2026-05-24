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
from sklearn.manifold import TSNE, trustworthiness

try:
    import umap
except ModuleNotFoundError as exc:  # pragma: no cover - runtime dependency guard
    raise ModuleNotFoundError(
        "Missing python dependency `umap-learn`. Install with:\n"
        "uv pip install --python ../.venv/bin/python umap-learn"
    ) from exc


def to_plot_space(points: np.ndarray) -> np.ndarray:
    if points.ndim != 2:
        raise ValueError("points must be a 2D array")
    if points.shape[1] >= 2:
        return points[:, :2]
    if points.shape[1] == 1:
        return np.hstack([points, np.zeros((points.shape[0], 1), dtype=points.dtype)])
    raise ValueError("points must have at least one feature")


def condensed_pairwise_distances(points: np.ndarray) -> np.ndarray:
    n_samples = points.shape[0]
    distances: list[float] = []
    for i in range(n_samples):
        for j in range(i + 1, n_samples):
            distances.append(float(np.linalg.norm(points[i] - points[j])))
    return np.asarray(distances, dtype=np.float64)


def normalize_by_mean(values: np.ndarray) -> np.ndarray:
    if values.size == 0:
        return values.copy()
    mean_value = float(np.mean(values))
    if mean_value == 0:
        return values.copy()
    return values / mean_value


def pearson_correlation(a: np.ndarray, b: np.ndarray) -> float:
    if a.size == 0 or b.size == 0 or a.size != b.size:
        return 0.0
    if float(np.std(a)) == 0.0 or float(np.std(b)) == 0.0:
        return 0.0
    return float(np.corrcoef(a, b)[0, 1])


def build_method_result(
    method_name: str,
    vectors: np.ndarray,
    embedding: np.ndarray,
    normalized_high_condensed: np.ndarray,
    trust_neighbors: int,
) -> dict[str, Any]:
    low_condensed = condensed_pairwise_distances(embedding)
    normalized_low = normalize_by_mean(low_condensed)
    dist_corr = pearson_correlation(normalized_high_condensed, normalized_low)
    trust = float(trustworthiness(vectors, embedding, n_neighbors=trust_neighbors, metric="euclidean"))

    return {
        "name": method_name,
        "embedding": embedding.tolist(),
        "trustworthiness": trust,
        "distance_correlation": dist_corr,
    }


def compute_python_methods_snapshot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    vectors = np.asarray(payload["vectors"], dtype=np.float64)
    if vectors.ndim != 2 or vectors.shape[0] < 2:
        raise ValueError("payload.vectors must be a 2D array with at least 2 samples")

    tsne_perplexity = float(payload["tsne_perplexity"])
    trust_neighbors = int(payload.get("trust_neighbors", 10))
    if trust_neighbors >= vectors.shape[0]:
        trust_neighbors = max(1, vectors.shape[0] - 1)

    high_condensed = condensed_pairwise_distances(vectors)
    normalized_high = normalize_by_mean(high_condensed)

    pca = PCA(n_components=2, svd_solver="full", whiten=False)
    pca_embedding = np.asarray(pca.fit_transform(vectors), dtype=np.float64)

    tsne = TSNE(
        n_components=2,
        perplexity=tsne_perplexity,
        early_exaggeration=12.0,
        learning_rate="auto",
        max_iter=350,
        n_iter_without_progress=200,
        min_grad_norm=1e-7,
        init="pca",
        method="exact",
        random_state=42,
    )
    tsne_embedding = np.asarray(tsne.fit_transform(vectors), dtype=np.float64)

    umap_model = umap.UMAP(
        n_components=2,
        n_neighbors=15,
        min_dist=0.1,
        spread=1.0,
        n_epochs=None,
        learning_rate=1.0,
        negative_sample_rate=5,
        metric="euclidean",
        init="spectral",
        random_state=42,
        n_jobs=1,
    )
    umap_embedding = np.asarray(umap_model.fit_transform(vectors), dtype=np.float64)

    methods = [
        build_method_result("PCA", vectors, pca_embedding, normalized_high, trust_neighbors),
        build_method_result("t-SNE", vectors, tsne_embedding, normalized_high, trust_neighbors),
        build_method_result("UMAP", vectors, umap_embedding, normalized_high, trust_neighbors),
    ]
    return {"methods": methods}


def render_comparison_plot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    dataset_name = str(payload.get("dataset_name", "dataset"))
    methods = payload.get("methods")
    if not isinstance(methods, list) or len(methods) == 0:
        raise ValueError("payload.methods must be a non-empty list")

    n_methods = len(methods)
    fig, axes = plt.subplots(1, n_methods, figsize=(5 * n_methods, 5), dpi=150, squeeze=False)
    axes_row = axes[0]

    for idx, method in enumerate(methods):
        name = str(method["name"])
        embedding = to_plot_space(np.asarray(method["embedding"], dtype=np.float64))
        trust = float(method["trustworthiness"])
        dist_corr = float(method["distance_correlation"])

        colors = np.arange(embedding.shape[0], dtype=np.int64)
        ax = axes_row[idx]
        ax.scatter(
            embedding[:, 0],
            embedding[:, 1],
            c=colors,
            cmap="viridis",
            s=14,
            alpha=0.85,
            linewidths=0.0,
        )
        ax.set_title(f"{name}\nTrust={trust:.4f}  DistCorr={dist_corr:.4f}", fontsize=10)
        ax.set_xlabel("Dim 1")
        ax.set_ylabel("Dim 2")
        ax.grid(alpha=0.2, linewidth=0.5)

    fig.suptitle(f"{dataset_name}: PCA / t-SNE / UMAP comparison", fontsize=12)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / f"dr_compare_{dataset_name}.png")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)

    return {"output_file": str(output_path)}


def render_go_vs_python_plot_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    dataset_name = str(payload.get("dataset_name", "dataset"))
    go_methods = payload.get("go_methods")
    python_methods = payload.get("python_methods")
    if not isinstance(go_methods, list) or not isinstance(python_methods, list):
        raise ValueError("payload.go_methods and payload.python_methods must be lists")

    go_by_name = {str(item["name"]): item for item in go_methods}
    py_by_name = {str(item["name"]): item for item in python_methods}
    method_order = ["PCA", "t-SNE", "UMAP"]

    fig, axes = plt.subplots(2, len(method_order), figsize=(5 * len(method_order), 9), dpi=150, squeeze=False)

    for col_idx, method_name in enumerate(method_order):
        if method_name not in go_by_name or method_name not in py_by_name:
            raise ValueError(f"missing method in payload: {method_name}")

        go_item = go_by_name[method_name]
        py_item = py_by_name[method_name]

        go_embedding = to_plot_space(np.asarray(go_item["embedding"], dtype=np.float64))
        py_embedding = to_plot_space(np.asarray(py_item["embedding"], dtype=np.float64))
        colors = np.arange(go_embedding.shape[0], dtype=np.int64)

        go_ax = axes[0, col_idx]
        go_ax.scatter(
            go_embedding[:, 0],
            go_embedding[:, 1],
            c=colors,
            cmap="viridis",
            s=14,
            alpha=0.85,
            linewidths=0.0,
        )
        go_ax.set_title(
            f"Go {method_name}\n"
            f"Trust={float(go_item['trustworthiness']):.4f} "
            f"DistCorr={float(go_item['distance_correlation']):.4f}",
            fontsize=10,
        )
        go_ax.set_xlabel("Dim 1")
        go_ax.set_ylabel("Dim 2")
        go_ax.grid(alpha=0.2, linewidth=0.5)

        py_ax = axes[1, col_idx]
        py_ax.scatter(
            py_embedding[:, 0],
            py_embedding[:, 1],
            c=colors,
            cmap="viridis",
            s=14,
            alpha=0.85,
            linewidths=0.0,
        )
        py_ax.set_title(
            f"Python {method_name}\n"
            f"Trust={float(py_item['trustworthiness']):.4f} "
            f"DistCorr={float(py_item['distance_correlation']):.4f}",
            fontsize=10,
        )
        py_ax.set_xlabel("Dim 1")
        py_ax.set_ylabel("Dim 2")
        py_ax.grid(alpha=0.2, linewidth=0.5)

    fig.suptitle(f"{dataset_name}: Go vs Python for PCA / t-SNE / UMAP", fontsize=12)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / f"dr_compare_go_vs_python_{dataset_name}.png")
    if not output_path.is_absolute():
        output_path = BASE_DIR / output_path
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)

    return {"output_file": str(output_path)}


def stitch_comparison_images_from_payload(payload: dict[str, Any]) -> dict[str, Any]:
    datasets = payload.get("datasets")
    if not isinstance(datasets, list) or len(datasets) == 0:
        raise ValueError("payload.datasets must be a non-empty list")

    n_rows = len(datasets)
    fig, axes = plt.subplots(n_rows, 1, figsize=(14, 4.5 * n_rows), dpi=150, squeeze=False)

    for row_idx, dataset in enumerate(datasets):
        name = str(dataset.get("name", f"dataset_{row_idx + 1}"))
        image_file = dataset.get("image_file")
        if not image_file:
            raise ValueError(f"datasets[{row_idx}] missing image_file")

        image_path = Path(image_file)
        if not image_path.is_absolute():
            image_path = BASE_DIR / image_path
        if not image_path.exists():
            raise FileNotFoundError(f"comparison image not found: {image_path}")

        image = plt.imread(image_path)
        ax = axes[row_idx, 0]
        ax.imshow(image)
        ax.axis("off")
        ax.set_title(name, fontsize=11, pad=8)

    title = str(payload.get("title", "Dimensionality reduction comparison overview"))
    fig.suptitle(title, fontsize=13)
    fig.tight_layout()

    output_file = payload.get("output_file")
    output_path = Path(output_file) if output_file else (BASE_DIR / "testdata" / "dr_compare_overview.png")
    if not output_path.is_absolute():
        output_path = BASE_DIR / output_path
    output_path.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(output_path, bbox_inches="tight")
    plt.close(fig)

    return {"output_file": str(output_path)}


class TestDrComparePlot(unittest.TestCase):
    def test_compute_python_methods_snapshot_from_payload(self) -> None:
        rng = np.random.default_rng(42)
        vectors = rng.normal(size=(30, 6)).astype(np.float64)
        payload = {
            "vectors": vectors.tolist(),
            "tsne_perplexity": 10.0,
            "trust_neighbors": 5,
        }

        result = compute_python_methods_snapshot_from_payload(payload)
        methods = result["methods"]
        self.assertEqual(len(methods), 3)
        names = [method["name"] for method in methods]
        self.assertEqual(names, ["PCA", "t-SNE", "UMAP"])
        for method in methods:
            embedding = np.asarray(method["embedding"], dtype=np.float64)
            self.assertEqual(embedding.shape, (30, 2))
            self.assertTrue(np.isfinite(embedding).all())

    def test_render_comparison_plot_from_payload(self) -> None:
        with tempfile.TemporaryDirectory(prefix="dr-compare-") as temp_dir:
            output_path = Path(temp_dir) / "compare.png"
            payload = {
                "dataset_name": "smoke",
                "methods": [
                    {
                        "name": "PCA",
                        "embedding": [[0.0, 0.0], [1.0, 1.0], [1.5, -0.5]],
                        "trustworthiness": 0.9,
                        "distance_correlation": 0.7,
                    },
                    {
                        "name": "t-SNE",
                        "embedding": [[0.0, 1.0], [1.0, 0.0], [1.2, -0.4]],
                        "trustworthiness": 0.95,
                        "distance_correlation": 0.65,
                    },
                    {
                        "name": "UMAP",
                        "embedding": [[-0.1, 0.8], [0.8, -0.2], [1.0, -0.6]],
                        "trustworthiness": 0.93,
                        "distance_correlation": 0.68,
                    },
                ],
                "output_file": str(output_path),
            }

            result = render_comparison_plot_from_payload(payload)
            self.assertEqual(result["output_file"], str(output_path))
            self.assertTrue(output_path.exists())

    def test_render_go_vs_python_plot_from_payload(self) -> None:
        with tempfile.TemporaryDirectory(prefix="dr-go-vs-py-") as temp_dir:
            output_path = Path(temp_dir) / "go_vs_py.png"
            payload = {
                "dataset_name": "smoke",
                "go_methods": [
                    {"name": "PCA", "embedding": [[0.0, 0.0], [1.0, 1.0]], "trustworthiness": 0.9, "distance_correlation": 0.8},
                    {"name": "t-SNE", "embedding": [[0.2, 0.1], [0.8, 0.9]], "trustworthiness": 0.95, "distance_correlation": 0.7},
                    {"name": "UMAP", "embedding": [[-0.1, 0.2], [0.9, 0.7]], "trustworthiness": 0.93, "distance_correlation": 0.75},
                ],
                "python_methods": [
                    {"name": "PCA", "embedding": [[0.1, 0.1], [0.9, 0.9]], "trustworthiness": 0.88, "distance_correlation": 0.79},
                    {"name": "t-SNE", "embedding": [[0.3, 0.2], [0.7, 0.8]], "trustworthiness": 0.94, "distance_correlation": 0.69},
                    {"name": "UMAP", "embedding": [[-0.2, 0.3], [0.8, 0.6]], "trustworthiness": 0.92, "distance_correlation": 0.73},
                ],
                "output_file": str(output_path),
            }

            result = render_go_vs_python_plot_from_payload(payload)
            self.assertEqual(result["output_file"], str(output_path))
            self.assertTrue(output_path.exists())

    def test_stitch_comparison_images_from_payload(self) -> None:
        with tempfile.TemporaryDirectory(prefix="dr-compare-stitch-") as temp_dir:
            temp_path = Path(temp_dir)
            image_a = temp_path / "a.png"
            image_b = temp_path / "b.png"

            plt.figure(figsize=(2, 1), dpi=80)
            plt.plot([0, 1], [0, 1], color="blue")
            plt.tight_layout()
            plt.savefig(image_a, bbox_inches="tight")
            plt.close()

            plt.figure(figsize=(2, 1), dpi=80)
            plt.plot([0, 1], [1, 0], color="orange")
            plt.tight_layout()
            plt.savefig(image_b, bbox_inches="tight")
            plt.close()

            output_path = temp_path / "overview.png"
            payload = {
                "title": "overview",
                "datasets": [
                    {"name": "a", "image_file": str(image_a)},
                    {"name": "b", "image_file": str(image_b)},
                ],
                "output_file": str(output_path),
            }

            result = stitch_comparison_images_from_payload(payload)
            self.assertEqual(result["output_file"], str(output_path))
            self.assertTrue(output_path.exists())


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--snapshot-stdin",
        action="store_true",
        help="Read vectors payload from stdin JSON and output Python DR methods snapshot.",
    )
    parser.add_argument(
        "--plot-stdin",
        action="store_true",
        help="Read comparison payload from stdin JSON and render plot.",
    )
    parser.add_argument(
        "--plot-go-vs-python-stdin",
        action="store_true",
        help="Read Go/Python payload from stdin JSON and render comparison plot.",
    )
    parser.add_argument(
        "--stitch-stdin",
        action="store_true",
        help="Read stitch payload from stdin JSON and generate overview image.",
    )
    args, remaining = parser.parse_known_args()

    if args.snapshot_stdin:
        payload = json.load(sys.stdin)
        result = compute_python_methods_snapshot_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.plot_stdin:
        payload = json.load(sys.stdin)
        result = render_comparison_plot_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.plot_go_vs_python_stdin:
        payload = json.load(sys.stdin)
        result = render_go_vs_python_plot_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    if args.stitch_stdin:
        payload = json.load(sys.stdin)
        result = stitch_comparison_images_from_payload(payload)
        json.dump(result, sys.stdout, ensure_ascii=False)
        sys.stdout.write("\n")
        return

    unittest.main(argv=[sys.argv[0], *remaining])


if __name__ == "__main__":
    main()
