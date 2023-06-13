package openai

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	sbctlutil "github.com/replicatedhq/sbctl/pkg/util"
	"github.com/sashabaranov/go-openai"
)

type Client struct {
	openai.Client
	maxTokens int
}

type rw struct {
	client     *Client
	ctx        context.Context
	cancel     context.CancelFunc
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
}

// New returns a new Client.
func New(key string, maxTokens int) *Client {

	client := openai.NewClient(key)
	return &Client{
		Client:    *client,
		maxTokens: maxTokens,
	}
}

func (c *Client) GetKubectlCmd(issueContent string) (openai.ChatCompletionResponse, error) {
	userMessageForModel := fmt.Sprintf("Github Issue, generate five kubectl command for debuging: ####%s####", issueContent)

	resp, err := c.Client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "You are an kubenete expert that use kubectl command to debug issues. The github issue description will be delimited with #### characters.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userMessageForModel,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return openai.ChatCompletionResponse{}, err
	}

	return resp, nil
}

// Chat creates a new chat session.
func (c *Client) Chat(ctx context.Context) io.ReadWriter {
	ctx, cancel := context.WithCancel(ctx)
	rd, wr := io.Pipe()
	return &rw{
		client:     c,
		ctx:        ctx,
		cancel:     cancel,
		pipeReader: rd,
		pipeWriter: wr,
	}
}

// Read reads from the chat.
func (r *rw) Read(b []byte) (n int, err error) {
	if r.ctx.Err() != nil {
		return 0, r.ctx.Err()
	}
	return r.pipeReader.Read(b)
}

// Write writes to the chat.
func (r *rw) Write(b []byte) (n int, err error) {
	if r.ctx.Err() != nil {
		return 0, r.ctx.Err()
	}

	request := sbctlutil.GetGithubIssue()
	var completion openai.ChatCompletionResponse
	for {
		// Generate completion
		completion, err = r.client.GetKubectlCmd(request)
		if err != nil {
			// Rate limit error, wait and try again
			log.Println("openai: too many requests, waiting for 30 seconds...")
			select {
			case <-time.After(30 * time.Second):
			case <-r.ctx.Done():
				return 0, r.ctx.Err()
			}
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("openai: couldn't generate completion: %w", err)
		}
		break
	}

	if len(completion.Choices) == 0 {
		return 0, fmt.Errorf("openai: no choices")
	}
	response := completion.Choices[0].Message.Content
	log.Printf("openai: request tokens %d", completion.Usage.TotalTokens)

	// Write response to pipe
	go func() {
		response := response + "\n"
		if _, err := r.pipeWriter.Write([]byte(response)); err != nil {
			log.Println(fmt.Errorf("openai: failed to write to pipe: %w", err))
		}
	}()
	return len(b), nil
}

// Close closes the chat.
func (r *rw) Close() error {
	r.cancel()
	return r.pipeReader.Close()
}
