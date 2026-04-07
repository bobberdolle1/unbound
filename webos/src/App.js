/*
 * Unbound for WebOS — App root component
 * Sets up Spotlight (D-pad) navigation and Moonstone theming
 */

import kind from '@enact/core/kind';
import Panels from '@enact/ui/Panels';
import SpotlightRootDecorator from '@enact/spotlight/SpotlightRootDecorator';
import MoonstoneDecorator from '@enact/moonstone';

import UnboundPanel from './components/UnboundPanel';

// Apply Moonstone theme + Spotlight (D-pad navigation)
const AppDecorator = SpotlightRootDecorator(
  MoonstoneDecorator({
    noAnimation: false,
    overlay: false
  })
);

const App = () => {
  return (
    <AppDecorator id="app">
      <Panels pattern="activity">
        <UnboundPanel />
      </Panels>
    </AppDecorator>
  );
};

export default App;
