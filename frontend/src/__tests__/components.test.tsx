import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { DoodleCheckbox } from '../components/DoodleCheckbox';
import { DoodleSelect } from '../components/DoodleSelect';
import { PingChart } from '../components/PingChart';

describe('Frontend UI Components', () => {
  describe('DoodleCheckbox', () => {
    it('should render label and description correctly', () => {
      render(
        <DoodleCheckbox
          checked={false}
          onChange={() => {}}
          label="AutoStart"
          desc="Launch app on system startup"
        />
      );

      expect(screen.getByText('AutoStart')).toBeDefined();
      expect(screen.getByText('Launch app on system startup')).toBeDefined();
    });

    it('should trigger onChange callback on click and keyboard events', () => {
      const handleChange = vi.fn();
      render(
        <DoodleCheckbox
          checked={false}
          onChange={handleChange}
          label="TCP Timestamps"
          desc="Enable TCP timestamps"
        />
      );

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);
      expect(handleChange).toHaveBeenCalledTimes(1);

      fireEvent.keyDown(checkbox, { key: 'Enter' });
      expect(handleChange).toHaveBeenCalledTimes(2);

      fireEvent.keyDown(checkbox, { key: ' ' });
      expect(handleChange).toHaveBeenCalledTimes(3);
    });
  });

  describe('DoodleSelect', () => {
    it('should display selected value and toggle options dropdown', () => {
      const handleChange = vi.fn();
      const options = ['Profile A', 'Profile B', 'Profile C'];

      render(
        <DoodleSelect
          value="Profile A"
          options={options}
          onChange={handleChange}
        />
      );

      expect(screen.getByText('Profile A')).toBeDefined();

      // Click to open dropdown
      const combobox = screen.getByRole('combobox');
      fireEvent.click(combobox);
      expect(screen.getByText('Profile B')).toBeDefined();
      expect(screen.getByText('Profile C')).toBeDefined();

      // Select option
      fireEvent.click(screen.getByText('Profile B'));
      expect(handleChange).toHaveBeenCalledWith('Profile B');
    });
  });

  describe('PingChart', () => {
    it('should not render anything when history is empty', () => {
      const { container } = render(<PingChart history={[]} />);
      expect(container.firstChild).toBeNull();
    });

    it('should render SVG ping chart and latency statistics', () => {
      const history = [12, 15, 20, 18, 14];
      render(<PingChart history={history} />);

      expect(screen.getByText(/14 мс/i)).toBeDefined();
      expect(screen.getByText(/ср: 16 мс/i)).toBeDefined();
      expect(screen.getByText(/мин: 12 \/ макс: 20/i)).toBeDefined();
    });
  });
});
