package deptest

import (
	"fmt"
	"testing"

	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/golang/snappy"
)

// TestMsgpackSnappyCompression 测试不同大小的 ChatContextMessage 的 msgpack 序列化和 snappy 压缩效果
func TestMsgpackSnappyCompression(t *testing.T) {
	messageSizes := []int{100, 500, 1000, 2000, 5000, 10000}

	// 打印表头
	header := fmt.Sprintf("| %-10s | %-13s | %-13s | %-8s | %-8s |",
		"原始大小", "msgpack", "snappy", "压缩比", "节省")
	separator := "|------------|---------------|---------------|----------|----------|"
	fmt.Println(header)
	fmt.Println(separator)

	for _, size := range messageSizes {
		// 构造大对象
		msg := constructChatContextMessage(size)

		// msgpack 序列化
		msgpackData, err := msgpack.Marshal(msg)
		if err != nil {
			t.Fatalf("msgpack marshal failed for size %d: %v", size, err)
		}
		msgpackSize := len(msgpackData)

		// snappy 压缩
		compressed := snappy.Encode(nil, msgpackData)
		snappySize := len(compressed)

		// 计算压缩比和节省比例
		compressionRatio := float64(snappySize) / float64(msgpackSize)
		savings := (1.0 - compressionRatio) * 100

		// 打印数据行
		row := fmt.Sprintf("| %-10d | %-13d | %-13d | %6.1f%%  | %6.1f%%  |",
			size, msgpackSize, snappySize, compressionRatio*100, savings)
		fmt.Println(row)
	}
}

// TestMsgpackSnappyDecompression 验证压缩后可正确解压
func TestMsgpackSnappyDecompression(t *testing.T) {
	// 构造一个较大的消息
	msg := constructChatContextMessage(5000)

	// 序列化
	msgpackData, err := msgpack.Marshal(msg)
	if err != nil {
		t.Fatalf("msgpack marshal failed: %v", err)
	}

	// 压缩
	compressed := snappy.Encode(nil, msgpackData)

	// 解压
	decompressed, err := snappy.Decode(nil, compressed)
	if err != nil {
		t.Fatalf("snappy decompress failed: %v", err)
	}

	// 反序列化
	var decodedMsg cacheschema.ChatContextMessage
	err = msgpack.Unmarshal(decompressed, &decodedMsg)
	if err != nil {
		t.Fatalf("msgpack unmarshal failed: %v", err)
	}

	// 验证数据完整性
	if msg.Id != decodedMsg.Id {
		t.Errorf("Id mismatch: got %s, want %s", decodedMsg.Id, msg.Id)
	}
	if msg.CreatedAt != decodedMsg.CreatedAt {
		t.Errorf("CreatedAt mismatch: got %d, want %d", decodedMsg.CreatedAt, msg.CreatedAt)
	}
	if len(msg.Message) != len(decodedMsg.Message) {
		t.Errorf("Message length mismatch: got %d, want %d", len(decodedMsg.Message), len(msg.Message))
	}
	if string(msg.Message) != string(decodedMsg.Message) {
		t.Error("Message content mismatch")
	}

	t.Logf("✓ Decompression successful: original=%d, compressed=%d, ratio=%.2f%%",
		len(msgpackData), len(compressed), float64(len(compressed))/float64(len(msgpackData))*100)
}

// TestMsgpackSnappyBenchmark 对不同大小进行 benchmark
func TestMsgpackSnappyBenchmark(t *testing.T) {
	messageSizes := []int{100, 500, 1000, 2000, 5000, 10000}

	for _, size := range messageSizes {
		msg := constructChatContextMessage(size)

		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			// 预热
			for i := 0; i < 100; i++ {
				data, _ := msgpack.Marshal(msg)
				snappy.Encode(nil, data)
			}

			// 正式测试
			iterations := 1000
			for i := 0; i < iterations; i++ {
				data, _ := msgpack.Marshal(msg)
				snappy.Encode(nil, data)
			}
		})
	}
}

// constructChatContextMessage 构造指定 Message 大小的 ChatContextMessage
func constructChatContextMessage(messageSize int) *cacheschema.ChatContextMessage {
	// 生成指定大小的 message 内容（模拟 eino schema.Message 的 JSON）
	messageContent := make([]byte, messageSize)
	for i := range messageContent {
		messageContent[i] = byte('a' + (i % 26))
	}

	return &cacheschema.ChatContextMessage{
		Id:        uuid.NewV7().String(),
		CreatedAt: 1234567890,
		Message:   messageContent,
		Extra:     []byte("extra data"),
	}
}
