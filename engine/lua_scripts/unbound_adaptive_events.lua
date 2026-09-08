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
				print(string.format("[UNBOUND_EVENT] adaptive host=%s event=rotate from=%d to=%d strategy=%d", host, old_strat, hrec.nstrategy, hrec.nstrategy))
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
		local was_failure = (crec and crec.failure)
		local was_nocheck = (crec and crec.nocheck)
		local old_counter = (hrec and hrec.failure_counter) or 0

		local failed = orig_check(desync, hrec, crec)

		if crec and crec.failure and not was_failure then
			local current_count = (hrec and hrec.failure_counter) or (old_counter + 1)
			local threshold = tonumber(desync.arg.fails) or 3
			print(string.format("[UNBOUND_EVENT] adaptive host=%s event=failure_detected strategy=%d count=%d threshold=%d", host, strat, current_count, threshold))
		elseif crec and crec.nocheck and not was_nocheck and not (crec and crec.failure) then
			print(string.format("[UNBOUND_EVENT] adaptive host=%s event=success_detected strategy=%d", host, strat))
		end
		return failed
	end
end
