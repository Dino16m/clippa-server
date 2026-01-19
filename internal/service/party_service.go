package service

import (
	"sync"
	"time"

	"github.com/dino16m/clippa-server/internal/data"
	"github.com/sirupsen/logrus"
)

type PartyHandle struct {
	partyService *PartyService
	inbox        chan []byte
	id           string
	logger       *logrus.Logger
}

func (p *PartyHandle) HandleMessage(msg []byte) error {
	p.logger.Info("Received message")
	incomingType, err := getMessageType(msg)
	if err != nil {
		p.logger.WithError(err).Error("invalid message type")
		return ErrInvalidMessage
	}

	p.logger.WithField("msgType", incomingType).Info("Got message type")
	obj, err := validateMessage(incomingType, msg)
	if err != nil {
		p.logger.WithError(err).Error("invalid message")
		return ErrInvalidMessage
	}
	p.logger.WithField("msgType", incomingType).Info("validated message type")

	p.handleInternal(incomingType, obj)
	p.partyService.sendMessage(p.id, msg)
	return nil
}

func (p *PartyHandle) handleInternal(msgType MessageType, msg any) {
	if msgType == SetLeader {
		message := msg.(Message[SetLeaderData])
		err := p.partyService.setLeader(message.Data.Address)
		if err != nil {
			p.inbox <- ErrorMessage(ErrLeaderNotSet.Error())
		}
	}
	if msgType == Conclave {
		err := p.partyService.resetLeader()
		if err != nil {
			p.inbox <- ErrorMessage(ErrLeaderNotSet.Error())
		}
	}
}

func (p *PartyHandle) Leave() {
	p.logger.Info("leaving party")
	p.partyService.leave(p.id)
}

func (p *PartyHandle) ID() string {
	return p.id
}

func (p *PartyHandle) Inbox() <-chan []byte {
	return p.inbox
}

type PartyService struct {
	partyStore  *data.PartyStore
	partyId     string
	outboxes    map[string]chan []byte
	outboxMutex *sync.RWMutex
	logger      *logrus.Logger
}

func newPartyService(partyId string, partyStore *data.PartyStore, logger *logrus.Logger) *PartyService {
	return &PartyService{
		partyStore:  partyStore,
		partyId:     partyId,
		outboxes:    make(map[string]chan []byte),
		logger:      logger,
		outboxMutex: &sync.RWMutex{},
	}
}

func (p *PartyService) setLeader(address string) error {
	party, err := p.partyStore.Get(p.partyId)
	if err != nil {
		return err
	}
	if party.LeaderAddress == address {
		return nil
	}
	p.logger.WithField("party", p.partyId).WithField("leader", address).Info("Setting leader")
	party.LeaderAddress = address
	return p.partyStore.Update(party)
}

func (p *PartyService) resetLeader() error {
	party, err := p.partyStore.Get(p.partyId)
	if err != nil {
		return err
	}
	if party.LeaderAddress == "" {
		return nil
	}
	p.logger.WithField("party", p.partyId).Info("Resetting leader")
	party.LeaderAddress = ""
	return p.partyStore.Update(party)
}

func (p *PartyService) join(memberId string) *PartyHandle {
	outbox := make(chan []byte)
	p.lock("Joining party")
	p.outboxes[memberId] = outbox
	p.unlock("Joining party")

	handle := &PartyHandle{
		partyService: p,
		inbox:        outbox,
		id:           memberId,
		logger:       p.logger,
	}
	p.sendMessage(memberId, JoinedMessage(memberId))
	return handle
}

func (p *PartyService) leave(id string) {
	p.lock("Leaving party")
	delete(p.outboxes, id)
	p.unlock("Leaving party")

	p.sendMessage(id, LeftMessage(id))
}
func (p *PartyService) lock(msg string) {
	p.logger.Debugf("locking %s", msg)
	p.outboxMutex.Lock()
}

func (p *PartyService) unlock(msg string) {
	p.outboxMutex.Unlock()
	p.logger.Debugf("unlocked %s", msg)
}

func (p *PartyService) sendMessage(senderId string, msg []byte) {
	p.logger.Info("forwarding")
	p.outboxMutex.RLock()
	defer p.outboxMutex.RUnlock()
	p.logger.Infof("sending message to %d outboxes", len(p.outboxes)-1)
	for id, outbox := range p.outboxes {
		if id == senderId {
			continue
		}
		p.logger.Infof("forwarding message to %s", id)
		timer := time.NewTimer(time.Millisecond * 100)
		select {
		case outbox <- msg:
			p.logger.Infof("forwarded message to %s", id)
		case <-timer.C:
			p.logger.Infof("timed out forwarding message to %s", id)
			close(outbox)
			return
		}
	}
}

type PartyServiceProvider struct {
	parties      map[string]*PartyService
	partyStore   *data.PartyStore
	partiesMutex *sync.RWMutex
	logger       *logrus.Logger
}

func NewPartyServiceProvider(partyStore *data.PartyStore, logger *logrus.Logger) *PartyServiceProvider {
	return &PartyServiceProvider{
		parties:      map[string]*PartyService{},
		partiesMutex: &sync.RWMutex{},
		partyStore:   partyStore,
		logger:       logger,
	}
}

func (p *PartyServiceProvider) JoinParty(id string, memberId string) *PartyHandle {
	logger := p.logger.WithField("id", id)
	p.partiesMutex.RLock()
	party, ok := p.parties[id]
	p.partiesMutex.RUnlock()
	if ok {
		logger.Info("reusing existing party")
		return party.join(memberId)
	}

	p.partiesMutex.Lock()
	defer p.partiesMutex.Unlock()
	logger.Info("Creating new party service")
	party = newPartyService(id, p.partyStore, logger.Logger)
	p.parties[id] = party
	return party.join(memberId)
}
