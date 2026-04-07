/*
 * Unbound for WebOS — Main panel with CONNECT button and status
 * Fully navigable via D-pad (Spotlight)
 */

import {useCallback, useState, useEffect} from 'react';
import kind from '@enact/core/kind';
import Spotlight from '@enact/spotlight';
import Button from '@enact/moonstone/Button';
import BodyText from '@enact/moonstone/BodyText';
import Header from '@enact/moonstone/Header';
import Panel from '@enact/ui/Panels/Panel';
import {Col, Row} from '@enact/ui/Layout';

import UnboundService from '../services/UnboundService';
import css from './UnboundPanel.module.less';

const UnboundPanel = ({spotlightId, ...rest}) => {
  const [isConnected, setIsConnected] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [currentProfile, setCurrentProfile] = useState('default');
  const [statusText, setStatusText] = useState('Ready to connect');
  const [showSettings, setShowSettings] = useState(false);

  // Check initial state
  useEffect(() => {
    const checkState = async () => {
      try {
        const running = await UnboundService.isRunning();
        setIsConnected(running);
        setStatusText(running ? 'Unbound is active' : 'Ready to connect');
      } catch (e) {
        setStatusText('Service unavailable');
      }
    };
    checkState();
  }, []);

  const handleConnect = useCallback(async () => {
    if (isConnected) {
      // Disconnect
      setIsConnecting(true);
      setStatusText('Disconnecting...');
      try {
        await UnboundService.stop();
        setIsConnected(false);
        setStatusText('Ready to connect');
      } catch (e) {
        setStatusText('Error disconnecting');
      }
      setIsConnecting(false);
    } else {
      // Connect
      setIsConnecting(true);
      setStatusText('Starting Unbound...');
      try {
        await UnboundService.start(currentProfile);
        setIsConnected(true);
        setStatusText('Unbound is active — YouTube unblocked');
      } catch (e) {
        setStatusText('Connection failed');
      }
      setIsConnecting(false);
    }
  }, [isConnected, currentProfile]);

  const handleSettings = useCallback(() => {
    setShowSettings(!showSettings);
  }, [showSettings]);

  return (
    <Panel {...rest} spotlightId={spotlightId}>
      <Header title="Unbound" type="compact" />

      <div className={css.container}>
        <Col className={css.content}>
          {/* Status indicator */}
          <div className={css.statusRing}>
            <div className={`${css.statusDot} ${isConnected ? css.connected : ''}`} />
          </div>

          <BodyText className={css.statusText}>{statusText}</BodyText>

          {/* Main CONNECT button */}
          <Button
            className={`${css.connectButton} ${isConnected ? css.disconnect : ''}`}
            onClick={handleConnect}
            disabled={isConnecting}
            spotlightId="connect-btn"
            size="large"
          >
            {isConnecting ? 'PLEASE WAIT...' : isConnected ? 'DISCONNECT' : 'CONNECT'}
          </Button>

          {/* Profile selector */}
          <div className={css.profileSelector}>
            <BodyText className={css.profileLabel}>Profile:</BodyText>
            <Row className={css.profileButtons}>
              {['default', 'aggressive', 'lite'].map((profile, idx) => (
                <Button
                  key={profile}
                  className={currentProfile === profile ? css.profileActive : ''}
                  onClick={() => {
                    setCurrentProfile(profile);
                    if (!isConnected) {
                      setStatusText(`Profile: ${profile}`);
                    }
                  }}
                  spotlightId={`profile-${idx}`}
                  size="small"
                >
                  {profile.charAt(0).toUpperCase() + profile.slice(1)}
                </Button>
              ))}
            </Row>
          </div>

          {/* Settings toggle */}
          <Button
            className={css.settingsButton}
            onClick={handleSettings}
            spotlightId="settings-btn"
            size="small"
          >
            ⚙ Settings
          </Button>

          {/* Settings panel (shown inline) */}
          {showSettings && (
            <div className={css.settingsPanel}>
              <BodyText className={css.settingsTitle}>Settings</BodyText>
              <div className={css.settingsItem}>
                <BodyText>Engine: nfqws (netfilter queue)</BodyText>
              </div>
              <div className={css.settingsItem}>
                <BodyText>Mode: iptables NFQUEUE redirect</BodyText>
              </div>
              <div className={css.settingsItem}>
                <BodyText>Root access: Required</BodyText>
              </div>
              <Button
                onClick={async () => {
                  try {
                    const version = await UnboundService.getVersion();
                    setStatusText(`Engine version: ${version}`);
                  } catch (e) {
                    setStatusText('Unable to query engine');
                  }
                }}
                size="small"
              >
                Check Engine Status
              </Button>
            </div>
          )}
        </Col>
      </div>
    </Panel>
  );
};

export default UnboundPanel;
