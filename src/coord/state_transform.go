package coord

import geo "coord/geometry"
import cagent "coord/agent"

//     Turn() uint64
//     Position() *geo.Point
//     Energy() Energy
//     Alive() bool
//     Wait() uint16

type StateTransform struct {
    turn uint64
    pos *geo.Point
    energy cagent.Energy
    alive bool
    wait uint16
}

func transformFromState(state *cagent.AgentState) *StateTransform {
    self := new(StateTransform)
    self.turn = state.Turn
    self.pos = state.Position
    self.energy = state.GetInventory().Energy
    self.alive = state.Live
    self.wait = state.Wait
    return self
}

func (self *StateTransform) mv(move *cagent.Move) {
    self.pos.X += move.Position.X
    self.pos.Y += move.Position.Y
}

func (self *StateTransform) Turn() uint64 { return self.turn }
func (self *StateTransform) Position() *geo.Point { return self.pos }
func (self *StateTransform) Energy() cagent.Energy { return self.energy }
func (self *StateTransform) Alive() bool { return self.alive }
func (self *StateTransform) Wait() uint16 { return self.wait }