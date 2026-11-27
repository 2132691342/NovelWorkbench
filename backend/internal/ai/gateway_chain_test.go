// 模型网关与用量记账的回归：实例缓存键维度 → 流式超时策略 → 一次调用一条用量记录。
// 缓存键漏掉任一维度都会让换密钥后继续复用旧实例；流式客户端的整体超时会拦腰切断长流；
// 用量记成两条会让统计出现「有调用次数、token 全为 0」的伪数据。

package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeRecorder 收集用量记录（不连库）。
type fakeRecorder struct {
	records []UsageRecord
}

// RecordUsage 记录一次上报。
func (r *fakeRecorder) RecordUsage(_ context.Context, record UsageRecord) {
	r.records = append(r.records, record)
}

// fakeChatModel 可控的对话模型：分别给出流式分片与非流式回复。
type fakeChatModel struct {
	chunks []*schema.Message                     // 流式分片
	reply  *schema.Message                       // 非流式回复
	reader *schema.StreamReader[*schema.Message] // 自定义流（模拟中途出错）
}

// Generate 返回预置的非流式回复。
func (f *fakeChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return f.reply, nil
}

// Stream 返回预置的流。
func (f *fakeChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if f.reader != nil {
		return f.reader, nil
	}
	return schema.StreamReaderFromArray(f.chunks), nil
}

// chunkWithUsage 构造带用量元信息的分片。
func chunkWithUsage(content string, promptTokens, completionTokens int) *schema.Message {
	return &schema.Message{
		Role:    schema.Assistant,
		Content: content,
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens: promptTokens, CompletionTokens: completionTokens,
			TotalTokens: promptTokens + completionTokens,
		}},
	}
}

// testSpec 构造测试用供应商规格。
func testSpec() ProviderSpec {
	return ProviderSpec{ProviderULID: "PRV_1", ProviderName: "测试供应商", ChatModelName: "m1"}
}

// consumeStream 模拟调用方读完整条流并汇总用量（与 chat_stream / bootstrap 一致）。
func consumeStream(t *testing.T, stream *schema.StreamReader[*schema.Message]) (StreamUsage, error) {
	t.Helper()
	defer stream.Close()
	var usage StreamUsage
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return usage, nil
		}
		if err != nil {
			return usage, err
		}
		usage.Add(chunk)
	}
}

// TestModelGatewayChain 覆盖模型网关的实例构造与用量记账。
func TestModelGatewayChain(t *testing.T) {
	// 环节1：缓存键。密钥、协议、地址、模型、温度、输出上限、超时任一变化都必须换键。
	base := ProviderSpec{
		ProviderULID: "PRV_1", ProtocolCode: ProtocolOpenAI, BaseURL: "https://gw.example.com",
		APIKey: "key-a", ChatModelName: "m1", Temperature: 0.7, MaxTokens: 2048, Timeout: 120 * time.Second,
	}
	mutations := []func(spec *ProviderSpec){
		func(spec *ProviderSpec) { spec.APIKey = "key-b" },
		func(spec *ProviderSpec) { spec.ProtocolCode = ProtocolAnthropic },
		func(spec *ProviderSpec) { spec.BaseURL = "https://gw2.example.com" },
		func(spec *ProviderSpec) { spec.ChatModelName = "m2" },
		func(spec *ProviderSpec) { spec.Temperature = 0.9 },
		func(spec *ProviderSpec) { spec.MaxTokens = 4096 },
		func(spec *ProviderSpec) { spec.Timeout = 60 * time.Second },
	}
	for _, mutate := range mutations {
		changed := base
		mutate(&changed)
		if base.cacheKey() == changed.cacheKey() {
			t.Fatalf("配置变更后缓存键未变化，实例会被错误复用：%+v", changed)
		}
	}

	// 环节2：流式客户端。整体超时会切断长流，只能靠首包超时与调用方 ctx 控制。
	client := newStreamHTTPClient(45 * time.Second)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("应使用 *http.Transport，实际 %T", client.Transport)
	}
	if client.Timeout != 0 || transport.ResponseHeaderTimeout != 45*time.Second {
		t.Fatalf("流式客户端超时策略错误：整体 %v，首包 %v", client.Timeout, transport.ResponseHeaderTimeout)
	}

	// 环节3：流式成功的用量记账。取流阶段不得记账，读完流才记一次。
	recorder := &fakeRecorder{}
	aiClient := NewClient()
	aiClient.SetUsageRecorder(recorder)
	spec := testSpec()
	stream, err := aiClient.streamWith(context.Background(), spec, &fakeChatModel{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "你好"},
		chunkWithUsage("", 2214, 14), // 网关在末包给用量
	}}, nil)
	if err != nil {
		t.Fatalf("取流失败：%v", err)
	}
	if len(recorder.records) != 0 {
		t.Fatalf("取流阶段不应产生用量记录，实际 %d 条", len(recorder.records))
	}
	usage, err := consumeStream(t, stream)
	if err != nil {
		t.Fatalf("读流失败：%v", err)
	}
	aiClient.RecordStreamUsage(spec, usage.TokenUsage(), 1500*time.Millisecond)
	if len(recorder.records) != 1 {
		t.Fatalf("流式调用应恰好 1 条用量记录，实际 %d 条：%+v", len(recorder.records), recorder.records)
	}
	record := recorder.records[0]
	if !record.SuccessFlag || !record.UsageKnownFlag {
		t.Fatalf("成功且带 usage 的调用应标记成功+用量已知：%+v", record)
	}
	if record.PromptTokens != 2214 || record.CompletionTokens != 14 || record.LatencyMs != 1500 {
		t.Fatalf("token 与耗时记录错误：%+v", record)
	}

	// 环节4：流式失败仍记一次调用，标记失败且用量未知。
	failRecorder := &fakeRecorder{}
	failClient := NewClient()
	failClient.SetUsageRecorder(failRecorder)
	streamErr := errors.New("网关断开")
	reader, writer := schema.Pipe[*schema.Message](1)
	go func() {
		defer writer.Close()
		writer.Send(nil, streamErr)
	}()
	failStream, err := failClient.streamWith(context.Background(), spec, &fakeChatModel{reader: reader}, nil)
	if err != nil {
		t.Fatalf("取流失败：%v", err)
	}
	failUsage, recvErr := consumeStream(t, failStream)
	if !errors.Is(recvErr, streamErr) {
		t.Fatalf("应读到流错误，实际 %v", recvErr)
	}
	failClient.RecordStreamFailure(spec, failUsage.TokenUsage(), 800*time.Millisecond)
	if len(failRecorder.records) != 1 {
		t.Fatalf("失败的流式调用应恰好 1 条用量记录，实际 %d 条", len(failRecorder.records))
	}
	failRecord := failRecorder.records[0]
	if failRecord.SuccessFlag || failRecord.UsageKnownFlag {
		t.Fatalf("失败调用应标记失败且用量未知：%+v", failRecord)
	}
	if failRecord.PromptTokens != 0 || failRecord.CompletionTokens != 0 {
		t.Fatalf("未拿到 usage 时 token 应为 0：%+v", failRecord)
	}

	// 环节5：非流式成功同样只记一条。
	genRecorder := &fakeRecorder{}
	genClient := NewClient()
	genClient.SetUsageRecorder(genRecorder)
	if _, err := genClient.generateWith(context.Background(), spec,
		&fakeChatModel{reply: chunkWithUsage("正文", 100, 20)}, nil); err != nil {
		t.Fatalf("非流式调用失败：%v", err)
	}
	if len(genRecorder.records) != 1 {
		t.Fatalf("非流式调用应恰好 1 条用量记录，实际 %d 条", len(genRecorder.records))
	}
	genRecord := genRecorder.records[0]
	if !genRecord.SuccessFlag || !genRecord.UsageKnownFlag ||
		genRecord.PromptTokens != 100 || genRecord.CompletionTokens != 20 {
		t.Fatalf("非流式用量记录错误：%+v", genRecord)
	}

	// 环节6：分片用量汇总取各分片最大值，全程无用量的流返回 nil 交由调用方按未知上报。
	var aggregate StreamUsage
	aggregate.Add(nil)
	aggregate.Add(&schema.Message{Role: schema.Assistant, Content: "无元信息分片"})
	if aggregate.TokenUsage() != nil {
		t.Fatalf("全程无用量时应返回 nil，交由调用方按未知用量上报")
	}
	aggregate.Add(chunkWithUsage("a", 10, 1))
	aggregate.Add(chunkWithUsage("b", 10, 5))
	aggregated := aggregate.TokenUsage()
	if aggregated == nil || aggregated.PromptTokens != 10 || aggregated.CompletionTokens != 5 || aggregated.TotalTokens != 15 {
		t.Fatalf("分片用量汇总错误：%+v", aggregated)
	}
}
