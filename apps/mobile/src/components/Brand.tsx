import { Children } from 'react';
import { Image, StyleSheet, Text, View } from 'react-native';
import type { TextProps } from 'react-native';

export function BrandText({ children, brand = false, ...props }: TextProps & { brand?: boolean }) {
  return <Text {...props}>{Children.map(children, (child) => typeof child === 'string'
    ? brand ? child.split(/(keepup)/gi).map((part, index) => /^keepup$/i.test(part)
      ? <Text key={index} style={{ color: '#f8fafc' }}>keep<Text style={{ color: '#22c55e' }}>up</Text></Text>
      : part) : child.replace(/keepup/gi, 'keepup')
    : child)}</Text>;
}

export function BrandHeader({ large = false }: { large?: boolean }) {
  return <View style={styles.header}>
    <Image source={require('../../assets/icon.png')} accessible={false}
      style={{ width: large ? 44 : 40, height: large ? 44 : 40 }} resizeMode="contain" />
    <BrandText brand accessibilityRole="header" accessibilityLabel="keepup"
      style={[styles.wordmark, large && styles.large]}>keepup</BrandText>
  </View>;
}

const styles = StyleSheet.create({
  header: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  wordmark: { color: '#f8fafc', fontSize: 32, fontWeight: '800', letterSpacing: -1, flexShrink: 1 },
  large: { fontSize: 36 },
});
