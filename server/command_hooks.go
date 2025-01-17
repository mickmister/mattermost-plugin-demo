package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
)

const (
	commandTriggerCrash     = "crash"
	commandTriggerHooks     = "demo_plugin"
	commandTriggerDialog    = "dialog"
	commandTriggerEphemeral = "ephemeral"

	dialogElementNameNumber = "somenumber"
	dialogElementNameEmail  = "someemail"

	commandDialogHelp = "###### Interactive Dialog Slash Command Help\n" +
		"- `/dialog` - pen an Interactive Dialog. Once submitted, user-entered input is posted back into a channel.\n" +
		"- `/dialog no-elements` - Open an Interactive Dialog with no elements. Once submitted, user's action is posted back into a channel.\n" +
		"- `/dialog help` - Show this help text"
)

func (p *Plugin) registerCommands() error {
	if err := p.API.RegisterCommand(&model.Command{

		Trigger:          commandTriggerHooks,
		AutoComplete:     true,
		AutoCompleteHint: "(true|false)",
		AutoCompleteDesc: "Enables or disables the demo plugin hooks.",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandTriggerHooks)
	}

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          commandTriggerCrash,
		AutoComplete:     true,
		AutoCompleteHint: "",
		AutoCompleteDesc: "Crashes Demo Plugin",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandTriggerCrash)
	}

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          commandTriggerEphemeral,
		AutoComplete:     true,
		AutoCompleteHint: "",
		AutoCompleteDesc: "Demonstrates an ephemeral post capabilities.",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandTriggerEphemeral)
	}

	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          commandTriggerDialog,
		AutoComplete:     true,
		AutoCompleteDesc: "Open an Interactive Dialog.",
		DisplayName:      "Demo Plugin Command",
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandTriggerDialog)
	}

	return nil
}

func (p *Plugin) emitStatusChange() {
	configuration := p.getConfiguration()

	p.API.PublishWebSocketEvent("status_change", map[string]interface{}{
		"enabled": !configuration.disabled,
	}, &model.WebsocketBroadcast{})
}

// ExecuteCommand executes a command that has been previously registered via the RegisterCommand
// API.
//
// This demo implementation responds to a /demo_plugin command, allowing the user to enable
// or disable the demo plugin's hooks functionality (but leave the command and webapp enabled).
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	switch trigger {
	case commandTriggerCrash:
		return p.executeCommandCrash(), nil
	case commandTriggerHooks:
		return p.executeCommandHooks(args), nil
	case commandTriggerEphemeral:
		return p.executeCommandEphemeral(args), nil
	case commandTriggerDialog:
		return p.executeCommandDialog(args), nil

	default:
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         fmt.Sprintf("Unknown command: " + args.Command),
		}, nil
	}
}

func (p *Plugin) executeCommandCrash() *model.CommandResponse {
	go p.crash()
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         "Crashing plugin",
	}
}

func (p *Plugin) executeCommandHooks(args *model.CommandArgs) *model.CommandResponse {
	configuration := p.getConfiguration()

	if strings.HasSuffix(args.Command, "true") {
		if !configuration.disabled {
			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         "The demo plugin hooks are already enabled.",
			}
		}

		configuration.disabled = false
		p.emitStatusChange()

		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "Enabled demo plugin hooks.",
		}

	}

	if strings.HasSuffix(args.Command, "false") {
		if configuration.disabled {
			return &model.CommandResponse{
				ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
				Text:         "The demo plugin hooks are already disabled.",
			}
		}

		configuration.disabled = true
		p.emitStatusChange()

		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         "Disabled demo plugin hooks.",
		}
	}

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         fmt.Sprintf("Unknown command action: " + args.Command),
	}
}

func (p *Plugin) executeCommandEphemeral(args *model.CommandArgs) *model.CommandResponse {
	siteURL := *p.API.GetConfig().ServiceSettings.SiteURL

	post := &model.Post{
		ChannelId: args.ChannelId,
		Message:   "test ephemeral actions",
		Props: model.StringInterface{
			"attachments": []*model.SlackAttachment{{
				Actions: []*model.PostAction{{
					Integration: &model.PostActionIntegration{
						Context: model.StringInterface{
							"count": 0,
						},
						URL: fmt.Sprintf("%s/plugins/%s/ephemeral/update", siteURL, manifest.Id),
					},
					Type: model.POST_ACTION_TYPE_BUTTON,
					Name: "Update",
				}, {
					Integration: &model.PostActionIntegration{
						URL: fmt.Sprintf("%s/plugins/%s/ephemeral/delete", siteURL, manifest.Id),
					},
					Type: model.POST_ACTION_TYPE_BUTTON,
					Name: "Delete",
				}},
			}},
		},
	}
	_ = p.API.SendEphemeralPost(args.UserId, post)
	return &model.CommandResponse{}
}

func (p *Plugin) executeCommandDialog(args *model.CommandArgs) *model.CommandResponse {
	serverConfig := p.API.GetConfig()

	var dialogRequest model.OpenDialogRequest
	fields := strings.Fields(args.Command)
	command := ""
	if len(fields) == 2 {
		command = fields[1]
	}

	switch command {
	case "help":
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         commandDialogHelp,
		}
	case "":
		dialogRequest = model.OpenDialogRequest{
			TriggerId: args.TriggerId,
			URL:       fmt.Sprintf("%s/plugins/%s/dialog/1", *serverConfig.ServiceSettings.SiteURL, manifest.Id),
			Dialog:    getDialogWithSampleElements(),
		}
	case "no-elements":
		dialogRequest = model.OpenDialogRequest{
			TriggerId: args.TriggerId,
			URL:       fmt.Sprintf("%s/plugins/%s/dialog/2", *serverConfig.ServiceSettings.SiteURL, manifest.Id),
			Dialog:    getDialogWithoutElements(),
		}
	default:
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         fmt.Sprintf("Unknown command: " + command),
		}
	}

	if err := p.API.OpenInteractiveDialog(dialogRequest); err != nil {
		errorMessage := "Failed to open Interactive Dialog"
		p.API.LogError(errorMessage, "err", err.Error())
		return &model.CommandResponse{
			ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
			Text:         errorMessage,
		}
	}
	return &model.CommandResponse{}
}

func getDialogWithSampleElements() model.Dialog {
	return model.Dialog{
		CallbackId: "somecallbackid",
		Title:      "Test Title",
		IconURL:    "http://www.mattermost.org/wp-content/uploads/2016/04/icon.png",
		Elements: []model.DialogElement{{
			DisplayName: "Display Name",
			Name:        "realname",
			Type:        "text",
			Default:     "default text",
			Placeholder: "placeholder",
			HelpText:    "This a regular input in an interactive dialog triggered by a test integration.",
		}, {
			DisplayName: "Email",
			Name:        dialogElementNameEmail,
			Type:        "text",
			SubType:     "email",
			Placeholder: "placeholder@bladekick.com",
			HelpText:    "This a regular email input in an interactive dialog triggered by a test integration.",
		}, {
			DisplayName: "Number",
			Name:        dialogElementNameNumber,
			Type:        "text",
			SubType:     "number",
		}, {
			DisplayName: "Display Name Long Text Area",
			Name:        "realnametextarea",
			Type:        "textarea",
			Placeholder: "placeholder",
			Optional:    true,
			MinLength:   5,
			MaxLength:   100,
		}, {
			DisplayName: "User Selector",
			Name:        "someuserselector",
			Type:        "select",
			Placeholder: "Select a user...",
			HelpText:    "Choose a user from the list.",
			Optional:    true,
			MinLength:   5,
			MaxLength:   100,
			DataSource:  "users",
		}, {
			DisplayName: "Channel Selector",
			Name:        "somechannelselector",
			Type:        "select",
			Placeholder: "Select a channel...",
			HelpText:    "Choose a channel from the list.",
			Optional:    true,
			MinLength:   5,
			MaxLength:   100,
			DataSource:  "channels",
		}, {
			DisplayName: "Option Selector",
			Name:        "someoptionselector",
			Type:        "select",
			Placeholder: "Select an option...",
			HelpText:    "Choose an option from the list.",
			Options: []*model.PostActionOptions{{
				Text:  "Option1",
				Value: "opt1",
			}, {
				Text:  "Option2",
				Value: "opt2",
			}, {
				Text:  "Option3",
				Value: "opt3",
			}},
		}},
		SubmitLabel:    "Submit",
		NotifyOnCancel: true,
		State:          "somestate",
	}
}

func getDialogWithoutElements() model.Dialog {
	return model.Dialog{
		CallbackId:     "somecallbackid",
		Title:          "Sample Confirmation Dialog",
		IconURL:        "http://www.mattermost.org/wp-content/uploads/2016/04/icon.png",
		Elements:       nil,
		SubmitLabel:    "Confirm",
		NotifyOnCancel: true,
		State:          "somestate",
	}
}

func (p *Plugin) crash() {
	<-time.NewTimer(time.Second).C
	y := 0
	_ = 1 / y
}
