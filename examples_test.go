package live

// ExampleIslandEngine_broadcast demonstrates how to use broadcast operations.
//
// Broadcasting to all islands of a specific type:
//   engine.BroadcastToIslandType("counter", Event{
//       T:    "increment",
//       Data: []byte(`{"amount": 5}`),
//   })
//
// Broadcasting to a specific island ID across all sessions:
//   engine.BroadcastToIsland("counter-123", Event{
//       T:    "reset",
//       Data: []byte(`{}`),
//   })