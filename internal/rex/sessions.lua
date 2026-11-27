-- Lists every Rex session for rex-sesh: its ID, label, tab count, and the
-- directory its focused terminal last reported. A session whose view or
-- directory can't be read is still listed, without those fields.
local list, err = rex.call("session.list")
if not list then
  error(err)
end
local out = {}
for _, s in ipairs(list.sessions or {}) do
  local item = { session_id = s.session_id, label = s.label, windows = 0 }
  -- session.view needs an attached connection; attaching also returns it.
  local attached = rex.call("session.attach", { session_id = s.session_id })
  local view = attached and attached.view
  if view then
    item.windows = #(view.windows or {})
    for _, w in ipairs(view.windows or {}) do
      if w.window_id == view.active_window_id and w.focused_block_id then
        item.focused_block_id = w.focused_block_id
        -- Fails for a block that isn't a terminal; the path stays empty.
        local pwd = rex.call("block.method", {
          session_id = s.session_id,
          block_id = w.focused_block_id,
          method = "pwd",
        })
        if type(pwd) == "table" and type(pwd.pwd) == "string" then
          item.pwd = pwd.pwd
        end
      end
    end
  end
  out[#out + 1] = item
end
return { sessions = out }
