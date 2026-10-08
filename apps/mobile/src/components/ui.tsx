import type { ReactNode, Ref } from 'react';
import { StatusBar } from 'expo-status-bar';
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, StatusBar as NativeStatusBar, StyleSheet, TextInput, View } from 'react-native';
import { BrandHeader, BrandText as Text } from './Brand';
import type { TextInputProps } from 'react-native';

export function Screen({ children, scrollRef }: { children: ReactNode; scrollRef?: Ref<ScrollView> }) {
  return (
    <KeyboardAvoidingView style={styles.screen} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <StatusBar style="light" />
      <ScrollView ref={scrollRef} keyboardShouldPersistTaps="handled" contentContainerStyle={styles.content}>
        <BrandHeader />
        {children}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

export function ActionButton({ label, onPress, disabled = false, secondary = false }: {
  label: string; onPress: () => void; disabled?: boolean; secondary?: boolean;
}) {
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={label} accessibilityState={{ disabled }}
      disabled={disabled} onPress={onPress}
      style={({ pressed }) => [styles.button, secondary && styles.secondaryButton, disabled && styles.disabled, pressed && styles.pressed]}>
      <Text style={[styles.buttonText, secondary && styles.secondaryButtonText]}>{label}</Text>
    </Pressable>
  );
}

export function Field({ label, ...props }: TextInputProps & { label: string }) {
  return <View style={styles.field}>
    <Text style={styles.label}>{label}</Text>
    <TextInput accessibilityLabel={label} placeholderTextColor="#8c9ca7" autoCorrect={false}
      {...props} style={[styles.input, props.style]} />
  </View>;
}

export function ErrorMessage({ message }: { message: string | null }) {
  if (!message) return null;
  return <Text style={styles.error} accessibilityRole="alert" accessibilityLiveRegion="polite">{message}</Text>;
}

export const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#0c1014' },
  content: { flexGrow: 1, width: '100%', maxWidth: 560, alignSelf: 'center', paddingHorizontal: 24,
    paddingTop: (Platform.OS === 'android' ? NativeStatusBar.currentHeight ?? 0 : 44) + 24, paddingBottom: 48, gap: 20 },
  title: { color: '#f8fafc', fontSize: 24, fontWeight: '700' },
  text: { color: '#b5c2cb', fontSize: 16, lineHeight: 24 },
  label: { color: '#f8fafc', fontSize: 16, fontWeight: '600' },
  card: { backgroundColor: '#151b20', borderColor: '#34404a', borderWidth: 1, borderRadius: 20, padding: 20, gap: 16 },
  field: { gap: 8 },
  input: { minHeight: 48, borderColor: '#34404a', borderWidth: 1, borderRadius: 12, paddingHorizontal: 14,
    paddingVertical: 12, color: '#f8fafc', fontSize: 16, backgroundColor: '#0c1014' },
  button: { minHeight: 48, backgroundColor: '#22c55e', borderRadius: 12, padding: 14, alignItems: 'center', justifyContent: 'center' },
  buttonText: { color: '#072713', fontSize: 16, fontWeight: '700' },
  secondaryButton: { backgroundColor: '#20262d', borderColor: '#34404a', borderWidth: 1 },
  secondaryButtonText: { color: '#f8fafc' },
  disabled: { opacity: 0.55 },
  pressed: { opacity: 0.8 },
  error: { color: '#ffb7c0', backgroundColor: '#341e26', borderColor: '#88505e', borderWidth: 1,
    borderRadius: 12, padding: 14, fontSize: 16, lineHeight: 24 },
});
