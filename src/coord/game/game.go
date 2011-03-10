package game

import "coord/agent"
import "coord/config"

type GameState struct {
    Turn int
    Agents []*agent.Agent
    conf *config.Config
}

func NewGameState() *GameState {
    return &GameState{0, make([]*agent.Agent, 0), nil}
}

func (self *GameState) Configure(conf *config.Config) {
    self.conf = conf
}

func (self *GameState) ApplyMoves(moves []*agent.Move, agentStates []*agent.AgentState) {

}
