-- Returns the terminal size and the VT text of one block in a session, for
-- rex-sesh's pane preview. The text includes scrollback; rows says how many
-- of its last lines are on screen.
local sid, bid = rex.args.session_id, rex.args.block_id
local attached, err = rex.call("session.attach", { session_id = sid })
if not attached then
  error(err)
end
local size, serr = rex.call("block.method", { session_id = sid, block_id = bid, method = "size" })
if not size then
  error(serr)
end
local text, ferr = rex.call("block.method", {
  session_id = sid,
  block_id = bid,
  method = "format",
  args = { format = "vt" },
})
if not text then
  error(ferr)
end
return { rows = size.rows, content = text.content }
