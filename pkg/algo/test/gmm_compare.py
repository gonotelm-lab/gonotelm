#!/usr/bin/env python3
from __future__ import annotations

import atexit
import json
import os
import shutil
import tempfile
from pathlib import Path


def setup_mplconfigdir() -> None:
    if os.getenv("MPLCONFIGDIR"):
        return
    temp_dir = tempfile.mkdtemp(prefix="mplconfig-")
    os.environ["MPLCONFIGDIR"] = temp_dir
    atexit.register(lambda: shutil.rmtree(temp_dir, ignore_errors=True))


setup_mplconfigdir()

import matplotlib.pyplot as plt
import numpy as np
from scipy.optimize import linear_sum_assignment
from sklearn.decomposition import PCA
from sklearn.metrics import (
    adjusted_rand_score,
    calinski_harabasz_score,
    confusion_matrix,
    davies_bouldin_score,
    normalized_mutual_info_score,
    silhouette_score,
)
from sklearn.mixture import GaussianMixture

ROOT = Path(__file__).resolve().parent

FAKE_INPUT = ROOT / "gmm_compare_fake.json"
EMBED_INPUT = ROOT / "gmm_compare_embed.json"

FAKE_OUTPUT = ROOT / "gmm_compare_fake.png"
EMBED_OUTPUT = ROOT / "gmm_compare_embed.png"


def load_payload(path: Path) -> dict:
    if not path.exists():
        raise FileNotFoundError(
            f"missing compare input: {path}\n"
            "先执行: GMM_COMPARE_EXPORT=1 go test ./pkg/algo/test -run TestGMMCompareExport -count=1 -v"
        )
    return json.loads(path.read_text(encoding="utf-8"))


def align_sklearn_labels(go_labels: np.ndarray, sk_labels: np.ndarray, cluster_count: int) -> np.ndarray:
    cm = confusion_matrix(go_labels, sk_labels, labels=list(range(cluster_count)))
    row_ind, col_ind = linear_sum_assignment(cm.max() - cm)
    mapping = {col: row for row, col in zip(row_ind, col_ind)}
    return np.array([mapping.get(label, label) for label in sk_labels], dtype=int)


def align_sklearn_means(
    go_labels: np.ndarray, sk_labels: np.ndarray, sk_means: np.ndarray, cluster_count: int
) -> np.ndarray:
    cm = confusion_matrix(go_labels, sk_labels, labels=list(range(cluster_count)))
    row_ind, col_ind = linear_sum_assignment(cm.max() - cm)
    aligned = np.zeros_like(sk_means)
    for row, col in zip(row_ind, col_ind):
        aligned[row] = sk_means[col]
    return aligned


def safe_cluster_metric(fn, vectors: np.ndarray, labels: np.ndarray) -> float:
    unique = np.unique(labels)
    if len(unique) <= 1 or len(unique) >= len(labels):
        return float("nan")
    return float(fn(vectors, labels))


def build_metrics(payload: dict, vectors: np.ndarray, go_labels: np.ndarray, sk_labels: np.ndarray, model: GaussianMixture) -> dict:
    metrics = {
        "sample_count": int(vectors.shape[0]),
        "feature_dim": int(vectors.shape[1]),
        "cluster_count": int(payload["cluster_count"]),
        "ari_go_sk": float(adjusted_rand_score(go_labels, sk_labels)),
        "nmi_go_sk": float(normalized_mutual_info_score(go_labels, sk_labels)),
        "go_silhouette": safe_cluster_metric(silhouette_score, vectors, go_labels),
        "sk_silhouette": safe_cluster_metric(silhouette_score, vectors, sk_labels),
        "go_davies_bouldin": safe_cluster_metric(davies_bouldin_score, vectors, go_labels),
        "sk_davies_bouldin": safe_cluster_metric(davies_bouldin_score, vectors, sk_labels),
        "go_calinski_harabasz": safe_cluster_metric(calinski_harabasz_score, vectors, go_labels),
        "sk_calinski_harabasz": safe_cluster_metric(calinski_harabasz_score, vectors, sk_labels),
        "go_log_likelihood": float(payload["go_result"]["log_likelihood"]),
        "sk_log_likelihood": float(model.score(vectors) * vectors.shape[0]),
        "go_iterations": int(payload["go_result"]["iterations"]),
        "sk_iterations": int(model.n_iter_),
    }

    true_labels = payload.get("true_labels")
    if true_labels and len(true_labels) == len(go_labels):
        true_arr = np.asarray(true_labels, dtype=int)
        metrics["ari_true_go"] = float(adjusted_rand_score(true_arr, go_labels))
        metrics["ari_true_sk"] = float(adjusted_rand_score(true_arr, sk_labels))
    return metrics


def draw_figure(
    title: str,
    vectors: np.ndarray,
    go_labels: np.ndarray,
    sk_labels_aligned: np.ndarray,
    go_means: np.ndarray,
    sk_means_aligned: np.ndarray,
    metrics: dict,
    output_path: Path,
) -> None:
    pca2 = PCA(n_components=2, random_state=0)
    points_2d = pca2.fit_transform(vectors)
    go_means_2d = pca2.transform(go_means)
    sk_means_2d = pca2.transform(sk_means_aligned)

    fig, axes = plt.subplots(2, 2, figsize=(12.5, 9.5), dpi=140)

    def scatter_panel(ax, labels, means, name):
        ax.scatter(points_2d[:, 0], points_2d[:, 1], c=labels, cmap="tab10", s=16, alpha=0.82, linewidths=0)
        ax.scatter(
            means[:, 0],
            means[:, 1],
            marker="X",
            s=240,
            c=np.arange(means.shape[0]),
            cmap="tab10",
            edgecolor="black",
            linewidth=1.2,
        )
        ax.set_title(name)
        ax.set_xlabel("PCA-1")
        ax.set_ylabel("PCA-2")
        ax.grid(alpha=0.25)

    scatter_panel(axes[0, 0], go_labels, go_means_2d, "Go GMM")
    scatter_panel(axes[0, 1], sk_labels_aligned, sk_means_2d, "sklearn GMM")

    disagreement = (go_labels != sk_labels_aligned).astype(int)
    axes[1, 0].scatter(
        points_2d[:, 0],
        points_2d[:, 1],
        c=disagreement,
        cmap="coolwarm",
        s=16,
        alpha=0.82,
        linewidths=0,
    )
    axes[1, 0].set_title("Disagreement")
    axes[1, 0].set_xlabel("PCA-1")
    axes[1, 0].set_ylabel("PCA-2")
    axes[1, 0].grid(alpha=0.25)

    axes[1, 1].axis("off")
    lines = [
        f"Samples: {metrics['sample_count']}",
        f"Dim: {metrics['feature_dim']}",
        f"K: {metrics['cluster_count']}",
        f"ARI(go,sk): {metrics['ari_go_sk']:.4f}",
        f"NMI(go,sk): {metrics['nmi_go_sk']:.4f}",
        f"Go silhouette: {metrics['go_silhouette']:.4f}",
        f"sk silhouette: {metrics['sk_silhouette']:.4f}",
        f"Go DBI: {metrics['go_davies_bouldin']:.4f}",
        f"sk DBI: {metrics['sk_davies_bouldin']:.4f}",
        f"Go CH: {metrics['go_calinski_harabasz']:.2f}",
        f"sk CH: {metrics['sk_calinski_harabasz']:.2f}",
        f"Go LL: {metrics['go_log_likelihood']:.2f}",
        f"sk LL: {metrics['sk_log_likelihood']:.2f}",
    ]
    if "ari_true_go" in metrics:
        lines.insert(4, f"ARI(true,go): {metrics['ari_true_go']:.4f}")
        lines.insert(5, f"ARI(true,sk): {metrics['ari_true_sk']:.4f}")
    axes[1, 1].text(
        0.02,
        0.98,
        "\n".join(lines),
        va="top",
        ha="left",
        fontsize=10,
        bbox={"boxstyle": "round,pad=0.45", "facecolor": "white", "alpha": 0.92},
    )

    fig.suptitle(title, fontsize=14)
    fig.tight_layout()
    fig.savefig(output_path)
    plt.close(fig)


def compare_one(input_path: Path, output_path: Path) -> None:
    payload = load_payload(input_path)
    vectors = np.asarray(payload["vectors"], dtype=np.float64)
    go_labels = np.asarray(payload["go_result"]["labels"], dtype=int)
    go_means = np.asarray(payload["go_result"]["means"], dtype=np.float64)
    k = int(payload["cluster_count"])

    model = GaussianMixture(
        n_components=k,
        covariance_type="full",
        max_iter=int(payload["max_iterations"]),
        tol=float(payload["tolerance"]),
        reg_covar=float(payload["regularization"]),
        random_state=int(payload["seed"]),
        init_params="kmeans",
    )
    model.fit(vectors)
    sk_labels = model.predict(vectors)
    sk_means = np.asarray(model.means_, dtype=np.float64)

    sk_labels_aligned = align_sklearn_labels(go_labels, sk_labels, k)
    sk_means_aligned = align_sklearn_means(go_labels, sk_labels, sk_means, k)

    metrics = build_metrics(payload, vectors, go_labels, sk_labels, model)
    draw_figure(
        title=f"GMM Compare - {payload['name']}",
        vectors=vectors,
        go_labels=go_labels,
        sk_labels_aligned=sk_labels_aligned,
        go_means=go_means,
        sk_means_aligned=sk_means_aligned,
        metrics=metrics,
        output_path=output_path,
    )

    # Compare input json is temporary; keep only final result files.
    input_path.unlink(missing_ok=True)
    print(f"created: {output_path}")


def main() -> None:
    compare_one(FAKE_INPUT, FAKE_OUTPUT)
    compare_one(EMBED_INPUT, EMBED_OUTPUT)


if __name__ == "__main__":
    main()

