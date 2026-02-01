package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type MessageType string

const (
	Conclave          MessageType = "conclave"
	Inconclusive      MessageType = "inconclusive"
	Ping              MessageType = "ping"
	Pong              MessageType = "pong"
	Vote              MessageType = "vote"
	SetLeader         MessageType = "set-leader"
	LeaderElected     MessageType = "leader-elected"
	LeaderUnreachable MessageType = "leader-unreachable"
	Clipboard         MessageType = "clipboard"
	Joined            MessageType = "joined"
	Left              MessageType = "left"
	Error             MessageType = "error"
)

var (
	ErrInvalidMessage = errors.New("INVALID_MESSAGE")
	ErrLeaderNotSet   = errors.New("LEADER_NOT_SET")
)

type UnitData struct{}
type ErrorData struct {
	Error string `json:"error"`
}

type InconclusiveData struct {
	Generation string `json:"generation"`
}

type ConclaveData struct {
	Addresses  []string `json:"addresses"`
	Generation string   `json:"generation"`
}

type Ballot struct {
	Address       string `json:"address"`
	Reachable     bool   `json:"reachable"`
	LatencyMillis int64  `json:"latency"`
}

type VoteData struct {
	Ballots    []Ballot `json:"ballots"`
	Generation string   `json:"generation"`
}

type SetLeaderData struct {
	Address    string `json:"address"`
	Generation string `json:"generation"`
}

type ClipboardData struct {
	Content string `json:"content"`
}

type Message[T any] struct {
	Data        T           `json:"data"`
	Sender      string      `json:"sender"`
	MessageType MessageType `json:"messageType"`
	CreatedAt   int64       `json:"createdAt"`
}

func ErrorMessage(msg string) []byte {

	response := Message[ErrorData]{
		Data:        ErrorData{Error: msg},
		Sender:      "",
		MessageType: Error,
		CreatedAt:   time.Now().UTC().Unix(),
	}

	b, _ := json.Marshal(response)
	return b
}

func JoinedMessage(sender string) []byte {
	response := Message[UnitData]{
		Data:        UnitData{},
		Sender:      sender,
		MessageType: Joined,
		CreatedAt:   time.Now().UTC().Unix(),
	}

	b, _ := json.Marshal(response)
	return b
}

func LeftMessage(sender string) []byte {
	response := Message[UnitData]{
		Data:        UnitData{},
		Sender:      sender,
		MessageType: Left,
		CreatedAt:   time.Now().UTC().Unix(),
	}

	b, _ := json.Marshal(response)
	return b
}

func getMessageType(raw []byte) (MessageType, error) {
	var msg Message[any]
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return "", err
	}
	return MessageType(msg.MessageType), nil
}

func parseMessage[T any](raw []byte) (Message[T], error) {
	var msg Message[T]
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return Message[T]{}, err
	}
	return msg, nil
}

func validateMessage(msgType MessageType, raw []byte) (any, error) {
	switch msgType {
	case Conclave:
		return parseMessage[ConclaveData](raw)
	case Inconclusive:
		return parseMessage[InconclusiveData](raw)
	case Vote:
		return parseMessage[VoteData](raw)
	case Ping, Pong, Joined, Left, LeaderUnreachable:
		return parseMessage[UnitData](raw)
	case SetLeader, LeaderElected:
		return parseMessage[SetLeaderData](raw)
	case Clipboard:
		return parseMessage[ClipboardData](raw)
	case Error:
		return parseMessage[ErrorData](raw)
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}
}
