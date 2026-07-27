package ru.unbound.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import ru.unbound.app.vpn.UnboundVpnService
import ru.unbound.app.vpn.VpnState

/**
 * Unit tests verifying VpnState states and VpnService constants.
 */
class VpnStateUnitTest {

    @Test
    fun vpnServiceConstants_areValid() {
        assertTrue("PACKET_RELAY_IMPLEMENTED should be true", UnboundVpnService.PACKET_RELAY_IMPLEMENTED)
        assertEquals("ru.unbound.vpn_channel", UnboundVpnService.CHANNEL_ID)
        assertEquals(1500, UnboundVpnService.VPN_MTU)
        assertEquals("10.0.0.2", UnboundVpnService.TUN_IP)
    }

    @Test
    fun vpnState_stateMapping() {
        val disconnected: VpnState = VpnState.Disconnected
        val connecting: VpnState = VpnState.Connecting
        val connected: VpnState = VpnState.Connected
        val error: VpnState = VpnState.Error("Test Error")

        assertEquals(VpnState.Disconnected, disconnected)
        assertEquals(VpnState.Connecting, connecting)
        assertEquals(VpnState.Connected, connected)
        assertTrue(error is VpnState.Error && error.message == "Test Error")
    }
}
