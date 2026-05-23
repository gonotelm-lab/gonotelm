#!/usr/bin/env python3
from __future__ import annotations

import json
import sys

import matplotlib.pyplot as plt
import numpy as np
from scipy.optimize import linear_sum_assignment
from sklearn.decomposition import PCA
from sklearn.metrics import (
    adjusted_rand_score,
    confusion_matrix,
    normalized_mutual_info_score,
    silhouette_score,
)
from sklearn.mixture import GaussianMixture


def run(input_path: str) -> dict:
    with open(input_path, "r", encoding="utf-8") as f:
        payload = json.load(f)

    vectors = np.asarray(payload["vectors"], dtype=float)
    go_labels = np.asarray(payload["go_labels"], dtype=int)
    go_means = np.asarray(payload["go_means"], dtype=float)
    target_dim = int(payload["target_dim"])
    cluster_count = int(payload["cluster_count"])

    vectors_pca = PCA(n_components=target_dim, svd_solver="full", random_state=0).fit_transform(vectors)

    gmm = GaussianMixture(
        n_components=cluster_count,
        covariance_type="full",
        max_iter=int(payload["max_iterations"]),
        tol=float(payload["tolerance"]),
        reg_covar=float(payload["regularization"]),
        random_state=int(payload["seed"]),
        init_params="kmeans",
    )
    gmm.fit(vectors_pca)

    py_labels = gmm.predict(vectors_pca)
    py_means = np.asarray(gmm.means_, dtype=float)

    conf = confusion_matrix(go_labels, py_labels, labels=list(range(cluster_count)))
    row_ind, col_ind = linear_sum_assignment(conf.max() - conf)
    mapping = {col: row for row, col in zip(row_ind, col_ind)}
    py_labels_aligned = np.array([mapping.get(label, label) for label in py_labels], dtype=int)

    py_means_aligned = np.zeros_like(py_means)
    for row, col in zip(row_ind, col_ind):
        py_means_aligned[row] = py_means[col]

    pca2 = PCA(n_components=2, random_state=0)
    points_2d = pca2.fit_transform(vectors_pca)
    go_means_2d = pca2.transform(go_means)
    py_means_2d = pca2.transform(py_means_aligned)

    fig, axes = plt.subplots(2, 2, figsize=(12.5, 9.5), dpi=140)

    def scatter_panel(ax, labels, means, title):
        ax.scatter(
            points_2d[:, 0],
            points_2d[:, 1],
            c=labels,
            cmap="tab10",
            s=16,
            alpha=0.82,
            linewidths=0,
        )
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
        ax.set_title(title)
        ax.set_xlabel("PCA-1")
        ax.set_ylabel("PCA-2")
        ax.grid(alpha=0.25)

    scatter_panel(axes[0, 0], go_labels, go_means_2d, "Go GMM (after PCA64)")
    scatter_panel(axes[0, 1], py_labels_aligned, py_means_2d, "sklearn GMM (after PCA64)")

    disagreement = (go_labels != py_labels_aligned).astype(int)
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

    ari = float(adjusted_rand_score(go_labels, py_labels))
    nmi = float(normalized_mutual_info_score(go_labels, py_labels))
    go_sil = float(silhouette_score(vectors_pca, go_labels))
    py_sil = float(silhouette_score(vectors_pca, py_labels))

    axes[1, 1].axis("off")
    axes[1, 1].text(
        0.02,
        0.98,
        "\n".join(
            [
                f"Samples: {vectors.shape[0]}",
                f"Input Dim: {vectors.shape[1]}",
                f"PCA Dim: {target_dim}",
                f"K: {cluster_count}",
                f"ARI(go,py): {ari:.4f}",
                f"NMI(go,py): {nmi:.4f}",
                f"Go silhouette: {go_sil:.4f}",
                f"Py silhouette: {py_sil:.4f}",
                f"Py iterations: {int(gmm.n_iter_)}",
            ]
        ),
        va="top",
        ha="left",
        fontsize=10,
        bbox={"boxstyle": "round,pad=0.45", "facecolor": "white", "alpha": 0.92},
    )

    fig.suptitle("PCA64 + GMM Compare (Go vs sklearn)", fontsize=14)
    fig.tight_layout()
    fig.savefig(payload["output_figure_path"])
    plt.close(fig)

    return {
        "ari": ari,
        "nmi": nmi,
        "go_silhouette": go_sil,
        "py_silhouette": py_sil,
        "py_iterations": int(gmm.n_iter_),
    }


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: python pca_gmm.py <input-json-path>")
    result = run(sys.argv[1])
    print("RESULT_JSON:" + json.dumps(result))


if __name__ == "__main__":
    main()

