export CLUSTER_ENDPOINT="http://${ENV_GONOTELM_MILVUS_ADDR:-127.0.0.1:19530}"
export TOKEN="$ENV_GONOTELM_MILVUS_USERNAME:$ENV_GONOTELM_MILVUS_PASSWORD"

export DB_NAME="gonotelm"

# create a database
curl -XPOST \
--url "${CLUSTER_ENDPOINT}/v2/vectordb/databases/create" \
--header "Authorization: Bearer ${TOKEN}" \
--header "Content-Type: application/json" \
--data @- <<EOF
{
    "dbName": "${DB_NAME}"
}
EOF

echo "\n"

# create source docs collection
export collectionSchema='{
    "autoId": false,
    "fields": [
        {
            "fieldName": "id",
            "dataType": "VarChar",
            "isPrimary": true,
            "elementTypeParams": {
                "max_length": 36
            }
        },
        {
            "fieldName": "notebook_id",
            "dataType": "VarChar",
            "elementTypeParams": {
                "max_length": 36
            }
        },
        {
            "fieldName": "source_id",
            "dataType": "VarChar",
            "elementTypeParams": {
                "max_length": 36
            }
        },
        {
            "fieldName": "content",
            "dataType": "VarChar",
            "elementTypeParams": {
                "max_length": 2048,
                "enable_analyzer": true
            }
        },
        {
            "fieldName": "sparse_content",
            "dataType": "SparseFloatVector"
        },
        {
            "fieldName": "owner",
            "dataType": "VarChar",
            "elementTypeParams": {
                "max_length": 255
            }
        },
        {
            "fieldName": "embedding",
            "dataType": "FloatVector",
            "elementTypeParams": {
                "dim": 1024
            }
        }
    ],
    "functions": [
        {
            "name": "content_bm25_emb",
            "type": "BM25",
            "inputFieldNames": ["content"],
            "outputFieldNames": ["sparse_content"],
            "params": {}
        }
    ]
}
'

export collectionIndexParams='[
    {
        "fieldName": "embedding",
        "metricType": "COSINE",
        "indexName": "idx_embedding",
        "indexType": "AUTOINDEX"
    },
    {
        "fieldName": "sparse_content",
        "metricType": "BM25",
        "indexType": "AUTOINDEX",
        "indexName": "idx_sparse_content",
        "params": {}
    },
    {
        "fieldName": "notebook_id",
        "indexName": "idx_notebook_id",
        "indexType": "STL_SORT"
    },
    {
        "fieldName": "source_id",
        "indexName": "idx_source_id",
        "indexType": "STL_SORT"
    }
]'

curl -XPOST \
--url "${CLUSTER_ENDPOINT}/v2/vectordb/collections/create" \
--header "Authorization: Bearer ${TOKEN}" \
--header "Content-Type: application/json" \
--data @- <<EOF
{
    "dbName": "${DB_NAME}",
    "collectionName": "source_docs",
    "schema": $collectionSchema,
    "indexParams": $collectionIndexParams
}
EOF

echo "\n"

# create collection partitions: _p0000 ~ _p0015
for i in $(seq 0 15); do
    partition_name=$(printf "_p%04d" "${i}")
    curl -XPOST \
    --url "${CLUSTER_ENDPOINT}/v2/vectordb/partitions/create" \
    --header "Authorization: Bearer ${TOKEN}" \
    --header "Content-Type: application/json" \
    --data @- <<EOF
{
    "dbName": "${DB_NAME}",
    "collectionName": "source_docs",
    "partitionName": "${partition_name}"
}
EOF
    echo "\n"
done
