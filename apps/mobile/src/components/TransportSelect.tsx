import { useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { transportLabels, transportModes } from '../domain/routes';
import type { TransportMode } from '../domain/routes';
import { styles } from './ui';

export function TransportSelect({ value, onChange, disabled = false }: {
  value: TransportMode; onChange: (value: TransportMode) => void; disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  return <View style={styles.field}>
    <Text style={styles.label}>Transport mode</Text>
    <Pressable accessibilityRole="combobox" accessibilityLabel="Transport mode"
      accessibilityValue={{ text: transportLabels[value] }} accessibilityState={{ expanded: open && !disabled, disabled }}
      disabled={disabled} onPress={() => setOpen(!open)}
      style={[styles.input, local.trigger, disabled && styles.disabled]}>
      <Text style={styles.label}>{transportLabels[value]}</Text>
      <Text accessible={false} style={styles.text}>{open && !disabled ? '▴' : '▾'}</Text>
    </Pressable>
    {open && !disabled && <View style={local.options}>
      {transportModes.map((mode) => <Pressable key={mode} accessibilityRole="radio"
        accessibilityLabel={transportLabels[mode]} accessibilityState={{ checked: value === mode }}
        onPress={() => { onChange(mode); setOpen(false); }}
        style={[local.option, value === mode && local.selected]}>
        <Text style={styles.label}>{transportLabels[mode]}</Text>
      </Pressable>)}
    </View>}
  </View>;
}

const local = StyleSheet.create({
  trigger: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 },
  options: { borderColor: '#34404a', borderWidth: 1, borderRadius: 12, overflow: 'hidden', backgroundColor: '#20262d' },
  option: { minHeight: 48, justifyContent: 'center', paddingHorizontal: 14, paddingVertical: 12 },
  selected: { backgroundColor: '#173023' },
});
