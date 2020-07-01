package process_test

import (
	"math/rand"
	"testing/quick"
	"time"

	"github.com/renproject/hyperdrive/process"
	"github.com/renproject/hyperdrive/process/processutil"
	"github.com/renproject/id"
	"github.com/renproject/surge"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Process", func() {

	Context("when unmarshaling fuzz", func() {
		It("should not panic", func() {
			f := func(fuzz []byte) bool {
				msg := process.Process{}
				Expect(surge.FromBinary(fuzz, &msg)).ToNot(Succeed())
				return true
			}
			Expect(quick.Check(f, nil)).To(Succeed())
		})
	})

	Context("when marshaling and then unmarshaling", func() {
		It("should equal itself", func() {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			f := func(proposeLogs map[process.Round]process.Propose, prevoteLogs map[process.Round]map[id.Signatory]process.Prevote, precommitLogs map[process.Round]map[id.Signatory]process.Precommit, onceFlags map[process.Round]process.OnceFlag) bool {
				expected := process.Process{
					State: processutil.RandomState(r),
				}
				expected.ProposeLogs = proposeLogs
				expected.PrevoteLogs = prevoteLogs
				expected.PrecommitLogs = precommitLogs
				expected.OnceFlags = onceFlags
				data, err := surge.ToBinary(expected)
				Expect(err).ToNot(HaveOccurred())
				got := process.Process{}
				err = surge.FromBinary(data, &got)
				Expect(err).ToNot(HaveOccurred())
				Expect(got.State.Equal(&expected.State)).To(BeTrue())
				return true
			}
			Expect(quick.Check(f, nil)).To(Succeed())
		})
	})

	// L11:
	//	Function StartRound(round)
	//		currentRound ← round
	//		currentStep ← propose
	//		if proposer(currentHeight, currentRound) = p then
	//			if validValue != nil then
	//				proposal ← validValue
	//			else
	//				proposal ← getValue()
	//			broadcast〈PROPOSAL, currentHeight, currentRound, proposal, validRound〉
	//		else
	//			schedule OnTimeoutPropose(currentHeight, currentRound) to be executed after timeoutPropose(currentRound)
	Context("when starting a round", func() {
		It("should set the current round to that round and set the current step to proposing", func() {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			f := func() bool {
				round := processutil.RandomRound(r)
				p := process.New(id.NewPrivKey().Signatory(), 33, nil, nil, nil, nil, nil, nil, nil)
				p.StartRound(round)
				Expect(p.CurrentRound).To(Equal(round))
				Expect(p.CurrentStep).To(Equal(process.Proposing))
				return true
			}
			Expect(quick.Check(f, nil)).To(Succeed())
		})

		Context("when we are the proposer", func() {
			Context("when our valid value is non-nil", func() {
				It("should propose the valid value", func() {
					panic("unimplemented")
				})
			})

			Context("when our valid value is nil", func() {
				It("should propose a new value", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when we are not the proposer", func() {
			It("should schedule a propose timeout", func() {
				panic("unimplemented")
			})
		})
	})

	// L57:
	//	Function OnTimeoutPropose(height, round)
	//		if height = currentHeight ∧ round = currentRound ∧ currentStep = propose then
	//			broadcast〈PREVOTE, currentHeight, currentRound, nil〉
	//			currentStep ← prevote
	Context("when timing out on a propose", func() {
		Context("when the timeout is in the current height", func() {
			Context("when the timeout is in the current round", func() {
				Context("when we are in the proposing step", func() {
					It("should prevote nil and move to the prevoting step", func() {
						panic("unimplemented")
					})
				})

				Context("when we are not the proposing step", func() {
					It("should do nothing", func() {
						panic("unimplemented")
					})
				})
			})

			Context("when the timeout is not in the current round", func() {
				It("should do nothing", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when the timeout is not in the current height", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L61:
	//	Function OnTimeoutPrevote(height, round)
	//		if height = currentHeight ∧ round = currentRound ∧ currentStep = prevote then
	//			broadcast〈PRECOMMIT, currentHeight, currentRound, nil
	//			currentStep ← precommitting
	Context("when timing out on a prevote", func() {
		Context("when the timeout is for the current height", func() {
			Context("when the timeout is for the current round", func() {
				Context("when the timeout is for the current step", func() {
					It("should precommit nil and move to the precommitting step", func() {
						panic("unimplemented")
					})
				})

				Context("when the timeout is not for the current step", func() {
					It("should do nothing", func() {
						panic("unimplemented")
					})
				})
			})

			Context("when the timeout is not for the current round", func() {
				It("should do nothing", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when the timeout is not for the current height", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L65:
	//	Function OnTimeoutPrecommit(height, round)
	//		if height = currentHeight ∧ round = currentRound then
	//			StartRound(currentRound + 1)
	Context("when timing out on a precommit", func() {
		Context("when the timeout is for the current height", func() {
			Context("when the timeout is for the current round", func() {
				It("should start a new round by incrementing currentRound", func() {
					panic("unimplemented")
				})
			})

			Context("when the timeout is not for the current round", func() {
				It("should do nothing", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when the timeout is not for the current height", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L22:
	//  upon〈PROPOSAL, currentHeight, currentRound, v, −1〉from proposer(currentHeight, currentRound)
	//  while currentStep = propose do
	//      if valid(v) ∧ (lockedRound = −1 ∨ lockedValue = v) then
	//          broadcast〈PREVOTE, currentHeight, currentRound, id(v)
	//      else
	//          broadcast〈PREVOTE, currentHeight, currentRound, nil
	//      currentStep ← prevote
	Context("when receiving a propose", func() {
		Context("when the message sender is the correct proposer for this height and round", func() {
			Context("when we are in the propose step", func() {
				Context("when the propose message is valid", func() {
					Context("when the locked round is equal to -1", func() {
						It("should prevote the value and move to the precommitting step", func() {
							panic("unimplemented")
						})
					})

					Context("when the locked value is the propose value", func() {
						It("should prevote the value and move to the precommitting step", func() {
							panic("unimplemented")
						})
					})

					Context("when the locked value is not -1 and the locked value is not the propose value", func() {
						It("should prevote nil and move to the precommitting step", func() {
							panic("unimplemented")
						})
					})
				})

				Context("when the propose message is invalid", func() {
					It("should prevote nil and move to the precommitting step", func() {
						panic("unimplemented")
					})
				})
			})

			Context("when we are not in the propose step", func() {
				It("should do nothing", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when the message sender is not the correct proposer for this height and round", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L28:
	//
	//  upon〈PROPOSAL, currentHeight, currentRound, v, vr〉from proposer(currentHeight, currentRound) AND 2f+ 1〈PREVOTE, currentHeight, vr, id(v)〉
	//  while currentStep = propose ∧ (vr ≥ 0 ∧ vr < currentRound) do
	//      if valid(v) ∧ (lockedRound ≤ vr ∨ lockedValue = v) then
	//          broadcast〈PREVOTE, currentHeight, currentRound, id(v)〉
	//      else
	//          broadcast〈PREVOTE, currentHeight, currentRound, nil〉
	//      currentStep ← prevote
	Context("when receiving a propose and 2f+1 prevotes", func() {
		Context("when the message sender is the correct proposer for the given height and round", func() {
			Context("when we are in the proposing step", func() {
				Context("when the proposed valid round is valid", func() {
					Context("when the proposed valid round is less than the current round", func() {
						Context("when the proposed value is valid", func() {
							Context("when the proposed valid round is greater than our locked round, or the proposed value is our locked value", func() {
								It("should prevote for the proposed value", func() {
									panic("unimplemented")
								})
							})

							Context("when the proposed valid round is not greater than our locked round, and the proposed value is not our locked value", func() {
								It("should prevote nil", func() {
									panic("unimplemented")
								})
							})
						})

						Context("when the proposed value is not valid", func() {
							It("should prevote nil", func() {
								panic("unimplemented")
							})
						})
					})

					Context("when the proposed valid round is not less than the current round", func() {
						It("should do nothing", func() {
							panic("unimplemented")
						})
					})
				})

				Context("when the proposed valid round is not valid", func() {
					It("should do nothing", func() {
						panic("unimplemented")
					})
				})
			})

			Context("when we are not in the proposing step", func() {
				It("should do nothing", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when the message sender is not the correct proposer for the given height and round", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L34:
	//
	//  upon 2f+ 1〈PREVOTE, currentHeight, currentRound, ∗〉
	//  while currentStep = prevote for the first time do
	//      scheduleOnTimeoutPrevote(currentHeight, currentRound) to be executed after timeoutPrevote(currentRound)
	Context("when receiving 2f+1 prevotes", func() {
		Context("when we are in step prevote", func() {
			It("should schedule a prevote timeout for the current height and round", func() {
				panic("unimplemented")
			})
		})

		Context("when we are not in the step prevote", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L36:
	//
	//  upon〈PROPOSAL, currentHeight, currentRound, v, ∗〉from proposer(currentHeight, currentRound) AND 2f+ 1〈PREVOTE, currentHeight, currentRound, id(v)〉
	//  while valid(v) ∧ currentStep ≥ prevote for the first time do
	//      if currentStep = prevote then
	//          lockedValue ← v
	//          lockedRound ← currentRound
	//          broadcast〈PRECOMMIT, currentHeight, currentRound, id(v))〉
	//          currentStep ← precommit
	//      validValue ← v
	//      validRound ← currentRound
	Context("when receiving a propose and 2f+1 prevotes, for any locked round in the propose message", func() {
		Context("when the proposed value is valid", func() {
			Context("when we are at least in the prevoting step", func() {
				Context("when we are in the prevoting step", func() {
					It("should set the locked value to the proposed value and the locked round to the current round", func() {
						panic("unimplemented")
					})

					It("should broadcast a precommit for the proposed value and move to the precommitting step", func() {
						panic("unimplemented")
					})

					It("should set the value value to the proposed value and the valid round to the current round", func() {
						panic("unimplemented")
					})
				})

				Context("when we are in the precommitting step", func() {
					It("should set the valid value to the proposed value and the valid round to the current round", func() {
						panic("unimplemented")
					})
				})
			})

			Context("when we are in the proposing step", func() {
				It("should do nothing", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when the proposed value is not valid", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L44:
	//
	//  upon 2f+ 1〈PREVOTE, currentHeight, currentRound, nil〉
	//  while currentStep = prevote do
	//      broadcast〈PRECOMMIT, currentHeight, currentRound, nil〉
	//      currentStep ← precommit
	Context("when receiving 2f+1 nil prevotes", func() {
		Context("when we are in the prevote step", func() {
			It("should precommit nil and move to the precommitting step", func() {
				panic("unimplemented")
			})
		})

		Context("when we are not in the prevote step", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L47:
	//
	//  upon 2f+ 1〈PRECOMMIT, currentHeight, currentRound, ∗〉for the first time do
	//      scheduleOnTimeoutPrecommit(currentHeight, currentRound) to be executed after timeoutPrecommit(currentRound)
	Context("when receiving 2f+1 precommits", func() {
		It("should schedule a precommit timeout for the current height and round", func() {
			panic("unimplemented")
		})
	})

	// L49:
	//
	//  upon〈PROPOSAL, currentHeight, r, v, ∗〉from proposer(currentHeight, r) AND 2f+ 1〈PRECOMMIT, currentHeight, r, id(v)〉
	//  while decision[currentHeight] = nil do
	//      if valid(v) then
	//          decision[currentHeight] = v
	//          currentHeight ← currentHeight + 1
	//          reset
	//          StartRound(0)
	Context("when receiving a propose and 2f+1 precommits", func() {
		Context("when we have not finalised the given height", func() {
			Context("when the received propose value is valid", func() {
				It("should finalise the given height, increment the current height and start a new round", func() {
					panic("unimplemented")
				})
			})

			Context("when the received propose value is not valid", func() {
				It("should do nothing", func() {
					panic("unimplemented")
				})
			})
		})

		Context("when we have already finalised the given height", func() {
			It("should do nothing", func() {
				panic("unimplemented")
			})
		})
	})

	// L55:
	//
	//  upon f+ 1〈∗, currentHeight, r, ∗, ∗〉with r > currentRound do
	//      StartRound(r)
	Context("when receiving f+1 messages from a future round", func() {
		It("should a new round with the current round set as the given future round", func() {
			panic("unimplemented")
		})
	})
})
