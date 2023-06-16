package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	openai "github.com/replicatedhq/sbctl/pkg/openai"
	sbctlutil "github.com/replicatedhq/sbctl/pkg/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "gpt",
		Short:         "GPT",
		Long:          `GPT`,
		SilenceUsage:  true,
		SilenceErrors: false,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			apiKey, err := sbctlutil.GetOpenAIKey()
			if err != nil {
				return errors.Wrap(err, "failed to get openai key")
			}

			logger := createLogger("interaction.log")

			client := openai.New(apiKey, 5000)

			githubIssueURL := v.GetString("issue")

			if githubIssueURL == "" {
				return errors.New("issue is required")
			} else {
				issueContent := sbctlutil.GetGithubIssue()

				// fmt.Println(string(resp.Choices[0].Message.Content))
				scanner := bufio.NewScanner(os.Stdin)

				for {
					fmt.Print("You: ")
					if !scanner.Scan() {
						break
					}
					input := scanner.Text()
					escapedInput := strconv.Quote(input)
					logger.Printf(input)
					if strings.ToLower(input) == "exit" {
						break
					}

					if strings.ToLower(input) == "github:" {
						resp, err := client.GetKubectlCmd(issueContent)
						if err != nil {
							return errors.Wrap(err, "failed to get kubectl command")
						}

						message := resp.Choices[0].Message
						if message.Content != "" {
							logAndPrintResponse(logger, message.Content)
						}
					} else {
						resp, err := client.GetKubectlCmd(escapedInput)
						if err != nil {
							return errors.Wrap(err, "failed to get kubectl command")
						}

						message := resp.Choices[0].Message
						if message.Content != "" {
							logAndPrintResponse(logger, message.Content)
						}
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().StringP("issue", "i", "", "github issue URL")
	return cmd
}

func logAndPrintResponse(logger *log.Logger, message string) {
	logger.Printf("AI: %s\n", message)
	fmt.Printf("AI: \n%s\n", message)
}

func createLogger(logFile string) *log.Logger {
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	return log.New(file, "", log.LstdFlags)
}

func Chat(ctx context.Context) error {
	apiKey, err := sbctlutil.GetOpenAIKey()
	if err != nil {
		return errors.Wrap(err, "failed to get openai key")
	}

	client := openai.New(apiKey, 5000)
	chat := client.Chat(ctx)

	err1 := make(chan error)
	err2 := make(chan error)

	go func() {
		if _, err := io.Copy(os.Stdout, chat); err != nil {
			err1 <- fmt.Errorf("gpt: couldn't copy: %w", err)
		}
	}()
	go func() {
		if _, err := io.Copy(chat, os.Stdin); err != nil {
			err2 <- fmt.Errorf("gpt: couldn't copy: %w", err)
		}
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-err1:
		return err
	case err := <-err2:
		return err
	}
}
