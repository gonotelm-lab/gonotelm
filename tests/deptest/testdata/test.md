# 第一章：基础概念 (H1) (Kind=Heading)
这是第一章的导言，我们将介绍整个系统的理论基础。(Kind=Paragraph)

该理论起源于上世纪 80 年代，至今仍在不断演进。(Kind=Paragraph)

## 第一节：核心定义 (H2)
在深入细节之前，我们需要明确几个关键术语。(Kind=Paragraph)

- 术语 A：表示系统中的最小可调度单元(Kind=List)
- 术语 B：用于描述单元之间的通信协议

1. 有序项 1：用于验证 ordered list 也被识别为 List (Kind=List)
2. 有序项 2：最终输出会被统一为 "- " 前缀

### 第一节的第一小节：术语 A 的细节 (H3) (Kind=Heading)
术语 A 具有以下特性：(Kind=Paragraph)

- 无状态 (Kind=List)
- 可水平扩展

一个典型的 A **实例定义如下（围栏代码块**）： (Kind=Paragraph)

```json
{
  "id": "a1 (Kind=FencedCodeBlock)",
  "capacity": 100,
  "type": "A",
  "description": "这个是一个A实例的描述",
  "tags": ["a", "b", "c"]
}
```

下面是缩进代码块（CodeBlock）：(Kind=Paragraph)

    package main (Kind=CodeBlock)

    func main() {
        println("hello from indented code block")
    }

下面是引用块（Blockquote）：(Kind=Paragraph)

> 引用段落第一行 (Kind=Blockquote)
> 引用段落第二行
>
> - 引用中的列表项 A
> - 引用中的列表项 B

下面是 HTML 块（HTMLBlock）： (Kind=Paragraph)

<div class="note"> 
  <strong>提示：</strong> (Kind=HTMLBlock)。
</div>

结束了，没有更多了。 (Kind=Paragraph)

# 总结 (H1) (Kind=Heading)

这个是一个 markdown 的测试文件，Powered-by gonotelm. (Kind=Paragraph)
