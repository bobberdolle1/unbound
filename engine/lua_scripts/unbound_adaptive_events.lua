-- unbound_adaptive_events.lua
-- Structured Machine-Readable Event Emitter for UNBOUND Adaptive Mode
-- Emits structured markers using standard print() (stdout), which is active in winws2/nfqws2 without --debug=1.

local orig_circular = _G["circular"]
if orig_circular then
	_G["circular"] = function(ctx, desync)
		local hrec = automate_host_record(desync)
		local old_strat = (hrec and hrec.nstrategy) or 1
		local had_strat = (hrec and hrec.nstrategy ~= nil)

		local verdict = orig_circular(ctx, desync)

		if hrec and hrec.nstrategy then
			local host = (desync.track and desync.track.hostname) or (hrec and hrec.host) or "unknown"
			if not had_strat then
				print(string.format("[UNBOUND_EVENT] adaptive host=%s event=init strategy=1", host))
			elseif hrec.nstrategy ~= old_strat then
				print(string.format("[UNBOUND_EVENT] adaptive host=%s event=rotate strategy=%d", host, hrec.nstrategy))
			end
		end
		return verdict
	end
end

local orig_check = _G["automate_failure_check"]
if orig_check then
	_G["automate_failure_check"] = function(desync, hrec, crec)
		local host = (desync.track and desync.track.hostname) or (hrec and hrec.host) or "unknown"
		local strat = (hrec and hrec.nstrategy) or 1
		local was_nocheck = crec and crec.nocheck

		local failed = orig_check(desync, hrec, crec)

		if crec and crec.nocheck and not was_nocheck then
			if failed then
				print(string.format("[UNBOUND_EVENT] adaptive host=%s event=failure strategy=%d", host, strat))
			else
				print(string.format("[UNBOUND_EVENT] adaptive host=%s event=success strategy=%d", host, strat))
			end
		end
		return failed
	end
end
