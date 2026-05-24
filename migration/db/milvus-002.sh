export CLUSTER_ENDPOINT="http://${ENV_GONOTELM_MILVUS_ADDR:-127.0.0.1:19530}"
export TOKEN="$ENV_GONOTELM_MILVUS_USERNAME:$ENV_GONOTELM_MILVUS_PASSWORD"

export DB_NAME="gonotelm"
export SOURCE_DOCS_COLLECTION_NAME="source_docs"
export FIELD_CHUNK_POS="chunk_pos"
export INDEX_FIELD_CHUNK_POS="idx_chunk_pos"

describe_resp=$(curl -s -XPOST \
--url "${CLUSTER_ENDPOINT}/v2/vectordb/collections/describe" \
--header "Authorization: Bearer ${TOKEN}" \
--header "Content-Type: application/json" \
--data @- <<EOF
{
    "dbName": "${DB_NAME}",
    "collectionName": "${SOURCE_DOCS_COLLECTION_NAME}"
}
EOF
)

if echo "${describe_resp}" | grep -Eq "\"(fieldName|name)\"[[:space:]]*:[[:space:]]*\"${FIELD_CHUNK_POS}\""; then
    echo "[skip] field already exists: ${FIELD_CHUNK_POS}"
else
    echo "[apply] add field: ${FIELD_CHUNK_POS}"
    curl -XPOST \
    --url "${CLUSTER_ENDPOINT}/v2/vectordb/collections/fields/add" \
    --header "Authorization: Bearer ${TOKEN}" \
    --header "Content-Type: application/json" \
    --data @- <<EOF
{
    "dbName": "${DB_NAME}",
    "collectionName": "${SOURCE_DOCS_COLLECTION_NAME}",
    "schema": {
        "fieldName": "${FIELD_CHUNK_POS}",
        "dataType": "Int32",
        "nullable": true,
        "defaultValue": -1
    }
}
EOF
fi

echo "\n"

index_list_resp=$(curl -s -XPOST \
--url "${CLUSTER_ENDPOINT}/v2/vectordb/indexes/list" \
--header "Authorization: Bearer ${TOKEN}" \
--header "Content-Type: application/json" \
--data @- <<EOF
{
    "dbName": "${DB_NAME}",
    "collectionName": "${SOURCE_DOCS_COLLECTION_NAME}"
}
EOF
)

if echo "${index_list_resp}" | grep -Eq "\"${INDEX_FIELD_CHUNK_POS}\""; then
    echo "[skip] index already exists: ${INDEX_FIELD_CHUNK_POS}"
else
    echo "[apply] create index: ${INDEX_FIELD_CHUNK_POS}"
    curl -XPOST \
    --url "${CLUSTER_ENDPOINT}/v2/vectordb/indexes/create" \
    --header "Authorization: Bearer ${TOKEN}" \
    --header "Content-Type: application/json" \
    --data @- <<EOF
{
    "dbName": "${DB_NAME}",
    "collectionName": "${SOURCE_DOCS_COLLECTION_NAME}",
    "indexParams": [
        {
            "fieldName": "${FIELD_CHUNK_POS}",
            "indexName": "${INDEX_FIELD_CHUNK_POS}",
            "indexType": "STL_SORT"
        }
    ]
}
EOF
fi