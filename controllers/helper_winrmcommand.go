package controllers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/masterzen/winrm"
	"github.com/pkg/errors"

	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1beta1"
)

func createWinrmCmd(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.DirectCommand, error) {
	functionScript, err := getFuncScript(provider)
	if err != nil {
		return &winrm.DirectCommand{}, err
	}
	cmd, err := createWinrmPowershell(provider, log)
	if err != nil {
		return &winrm.DirectCommand{}, err
	}
	if err := sendWinrmFunctions(log, cmd, functionScript); err != nil {
		cmd.Close()
		return &winrm.DirectCommand{}, err
	}
	if err := sendWinrmConnect(log, cmd, provider.ScvmmHost); err != nil {
		cmd.Close()
		return &winrm.DirectCommand{}, err
	}
	return cmd, nil
}

func createWinrmConnection(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.Client, error) {
	defer winrmTimer("CreateConnection")()
	endpoint := winrm.NewEndpoint(provider.ExecHost, 5985, false, false, nil, nil, nil, 0)
	// Don't use winrm.DefaultParameters here because of concurrency issues
	params := winrm.NewParameters("PT60S", "en-US", 153600)
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }
	params.RequestOptions = map[string]string{
		"WINRS_CODEPAGE":          "65001",
		"WINRS_NOPROFILE":         "TRUE",
		"WINRS_CONSOLEMODE_STDIN": "FALSE",
		"WINRS_SKIP_CMD_SHELL":    "TRUE",
	}

	if ExtraDebug {
		log.V(1).Info("Creating WinRM connection", "host", provider.ExecHost, "port", 5985)
	}
	winrmClient, err := winrm.NewClientWithParameters(endpoint, provider.ScvmmUsername, provider.ScvmmPassword, params)
	if err != nil {
		winrmErrors.WithLabelValues("CreateConnection").Inc()
		return nil, errors.Wrap(err, "Creating winrm client")
	}
	return winrmClient, nil
}

func createWinrmShell(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.Shell, error) {
	winrmClient, err := createWinrmConnection(provider, log)
	if err != nil {
		return nil, err
	}
	defer winrmTimer("CreateShell")()
	if ExtraDebug {
		log.V(1).Info("Creating WinRM shell")
	}
	shell, err := winrmClient.CreateShell()
	if err != nil {
		winrmErrors.WithLabelValues("CreateShell").Inc()
		return nil, errors.Wrap(err, "Creating winrm shell")
	}
	return shell, nil
}

func createWinrmPowershell(provider *infrav1.ScvmmProviderSpec, log logr.Logger) (*winrm.DirectCommand, error) {
	shell, err := createWinrmShell(provider, log)
	if err != nil {
		return nil, err
	}
	if ExtraDebug {
		log.V(1).Info("Starting WinRM powershell.exe")
	}
	defer winrmTimer("powershell.exe")()
	cmd, err := shell.ExecuteDirect("powershell.exe", "-NonInteractive", "-NoProfile", "-Command", "-")
	if err != nil {
		winrmErrors.WithLabelValues("powershell.exe").Inc()
		return nil, errors.Wrap(err, "Creating winrm powershell")
	}
	if err := sendWinrmPing(log, cmd, "Creating winrm powershell"); err != nil {
		winrmErrors.WithLabelValues("powershell.exe").Inc()
		return nil, err
	}
	return cmd, nil
}

func sendWinrmPing(log logr.Logger, cmd *winrm.DirectCommand, what string) error {
	log.V(1).Info("Sending WinRM ping")
	if err := cmd.SendCommand("Write-Host 'OK'"); err != nil {
		return errors.Wrap(err, what+", Pinging")
	}
	log.V(1).Info("Getting WinRM ping")
	stdout, stderr, _, _, err := cmd.ReadOutput()
	if err != nil {
		return errors.Wrap(err, what+", Reading result")
	}
	log.V(1).Info("Got WinRM ping", "stdout", string(stdout), "stderr", string(stderr))
	if strings.TrimSpace(string(stdout)) != "OK" {
		return errors.New(what + " result: " + string(stdout) + " (ERR=" + string(stderr))
	}
	return nil
}

func sendWinrmFunctions(log logr.Logger, cmd *winrm.DirectCommand, functionScript []byte) error {
	if ExtraDebug {
		log.V(1).Info("Sending WinRM function script")
	}
	defer winrmTimer("SendFunctions")()
	if err := cmd.SendInput(functionScript, false); err != nil {
		winrmErrors.WithLabelValues("SendFunctions").Inc()
		return errors.Wrap(err, "Sending powershell functions")
	}
	if err := sendWinrmPing(log, cmd, "Sending powerwhell functions"); err != nil {
		winrmErrors.WithLabelValues("SendFunctions").Inc()
		return err
	}
	return nil
}

func sendWinrmConnect(log logr.Logger, cmd *winrm.DirectCommand, scvmmHost string) error {
	defer winrmTimer("ConnectSCVMM")()
	if ExtraDebug {
		log.V(1).Info("Calling WinRM function ConnectSCVMM")
	}
	if err := cmd.SendCommand("ConnectSCVMM -Computername '%s'", scvmmHost); err != nil {
		winrmErrors.WithLabelValues("ConnectSCVMM").Inc()
		return errors.Wrap(err, "Connecting to SCVMM")
	}
	if err := sendWinrmPing(log, cmd, "Connecting to SCVMM"); err != nil {
		winrmErrors.WithLabelValues("ConnectSCVMM").Inc()
		return err
	}
	return nil
}

func getWinrmResult(cmd *winrm.DirectCommand, funcName string, log logr.Logger) (VMResult, error) {
	stdout, stderr, _, _, err := cmd.ReadOutput()
	if err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMResult{}, errors.Wrap(err, "Failed to read output")
	}
	if ExtraDebug {
		log.V(1).Info("Got WinRM Result", "stdout", string(stdout), "stderr", string(stderr))
	}
	var res VMResult
	if err := json.Unmarshal(stdout, &res); err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMResult{}, errors.Wrap(err, "Decode result error: "+string(stdout)+
			"  (stderr="+string(stderr)+")")
	}
	if res.Error != "" {
		err := &ScriptError{function: funcName, message: res.Message}
		log.V(1).Error(err, "Script error", "function", funcName, "stacktrace", res.Error)
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMResult{}, err
	}
	log.V(1).Info(funcName+" Result", "vm", res)
	return res, nil
}

func sendWinrmCommand(log logr.Logger, cmd *winrm.DirectCommand, command string, args ...interface{}) (VMResult, error) {
	if ExtraDebug {
		log.V(1).Info("Sending WinRM command", "command", command, "args", args,
			"cmdline", fmt.Sprintf(command+"\n", args...))
	}
	funcName, _, _ := strings.Cut(command, " ")
	log.V(1).Info("Call " + funcName)
	defer winrmTimer(funcName)()
	if err := cmd.SendCommand(command, args...); err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMResult{}, err
	}
	return getWinrmResult(cmd, funcName, log)
}

func winrmTimer(funcName string) func() {
	winrmTotal.WithLabelValues(funcName).Inc()
	start := time.Now()
	return func() {
		winrmDuration.WithLabelValues(funcName).Observe(time.Since(start).Seconds())
	}
}

func getWinrmSpecResult(cmd *winrm.DirectCommand, funcName string, log logr.Logger) (VMSpecResult, error) {
	stdout, stderr, _, _, err := cmd.ReadOutput()
	if err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMSpecResult{}, errors.Wrap(err, "Failed to read output")
	}
	if ExtraDebug {
		log.V(1).Info("Got WinRMSpec Result", "stdout", string(stdout), "stderr", string(stderr))
	}
	var res VMSpecResult
	if err := json.Unmarshal(stdout, &res); err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMSpecResult{}, errors.Wrap(err, "Decode result error: "+string(stdout)+
			"  (stderr="+string(stderr)+")")
	}
	if res.Error != "" {
		err := &ScriptError{function: funcName, message: res.Message}
		log.V(1).Error(err, "Script error", "function", funcName, "stacktrace", res.Error)
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMSpecResult{}, err
	}
	return res, nil
}

func sendWinrmSpecCommand(log logr.Logger, cmd *winrm.DirectCommand, command string, scvmmMachine *infrav1.ScvmmMachine) (VMSpecResult, error) {
	specjson, err := json.Marshal(scvmmMachine.Spec)
	if err != nil {
		return VMSpecResult{}, errors.Wrap(err, "encoding spec")
	}
	metajson, err := json.Marshal(scvmmMachine.ObjectMeta)
	if err != nil {
		return VMSpecResult{}, errors.Wrap(err, "encoding metadata")
	}
	if ExtraDebug {
		log.V(1).Info("Sending WinRM command", "command", command, "spec", scvmmMachine.Spec,
			"metadata", scvmmMachine.ObjectMeta,
			"cmdline", fmt.Sprintf(command+" -spec '%s' -metadata '%s'\n",
				escapeSingleQuotes(string(specjson)),
				escapeSingleQuotes(string(metajson))))
	}
	funcName, _, _ := strings.Cut(command, " ")
	defer winrmTimer(funcName)()
	if err := cmd.SendCommand(command+" -spec '%s' -metadata '%s'",
		escapeSingleQuotes(string(specjson)),
		escapeSingleQuotes(string(metajson))); err != nil {
		winrmErrors.WithLabelValues(funcName).Inc()
		return VMSpecResult{}, errors.Wrap(err, "sending command")
	}
	return getWinrmSpecResult(cmd, funcName, log)
}
