-- Lists every Rex session for rex-sesh: its ID, label, tab count, and the
-- directory a terminal in its active tab last reported. A session whose view
-- or directory can't be read is still listed, without those fields.
--
-- rex.args.block_id names the caller's own block. The session that holds it
-- is marked current. A caller in a floating layer never supplies the
-- directory: a picker in a focused floating layer is its tab's focused block.
local self_block = (rex.args and rex.args.block_id) or ""

-- The kind of layer that holds block_id in view, or nil.
local function layer_kind(view, block_id)
  for _, w in ipairs(view.windows or {}) do
    for _, layer in ipairs(w.layers or {}) do
      for _, b in ipairs(layer.blocks or {}) do
        if b.block_id == block_id then
          return layer.kind
        end
      end
    end
  end
  return nil
end

-- The focused block first, then the tiled blocks, skipping skip.
local function candidates(window, skip)
  local out = {}
  if window.focused_block_id and window.focused_block_id ~= skip then
    out[#out + 1] = window.focused_block_id
  end
  for _, layer in ipairs(window.layers or {}) do
    if layer.kind == "tiled" then
      for _, b in ipairs(layer.blocks or {}) do
        if b.block_id ~= skip and b.block_id ~= window.focused_block_id then
          out[#out + 1] = b.block_id
        end
      end
    end
  end
  return out
end

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
    local self_kind = self_block ~= "" and layer_kind(view, self_block) or nil
    item.current = self_kind ~= nil
    local skip = self_kind == "floating" and self_block or ""
    for _, w in ipairs(view.windows or {}) do
      if w.window_id == view.active_window_id then
        for _, block_id in ipairs(candidates(w, skip)) do
          -- Fails, or reports nothing, for a block that isn't a shell.
          local pwd = rex.call("block.method", {
            session_id = s.session_id,
            block_id = block_id,
            method = "pwd",
          })
          if type(pwd) == "table" and type(pwd.pwd) == "string" and pwd.pwd ~= "" then
            item.focused_block_id = block_id
            item.pwd = pwd.pwd
            break
          end
        end
      end
    end
  end
  out[#out + 1] = item
end
return { sessions = out }
