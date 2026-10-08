import { useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { AccessibilityInfo, Animated, PanResponder, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { BrandText as Text } from './Brand';
import { styles } from './ui';

// Only the handle owns drag gestures. The map and drawer content keep their own responders.
export function RouteDrawer({ expanded, onExpandedChange, height, bottomInset, summary, children, onPeekHeight }: {
  expanded: boolean; onExpandedChange: (expanded: boolean) => void; height: number; bottomInset: number;
  summary: ReactNode; children: ReactNode; onPeekHeight: (height: number) => void;
}) {
  const [summaryHeight, setSummaryHeight] = useState(120);
  const [dragging, setDragging] = useState(false);
  const [reduceMotion, setReduceMotion] = useState(true);
  const translate = useRef(new Animated.Value(Math.max(0, height - 48 - 120 - bottomInset))).current;
  const position = useRef(Math.max(0, height - 48 - 120 - bottomInset));
  const dragStart = useRef(0);
  const scroll = useRef<ScrollView>(null);
  const peekHeight = Math.min(height, 48 + summaryHeight + bottomInset);
  const travel = Math.max(0, height - peekHeight);
  const current = useRef({ expanded, travel, onExpandedChange });
  current.current = { expanded, travel, onExpandedChange };

  useEffect(() => {
    let active = true;
    void AccessibilityInfo.isReduceMotionEnabled().then((value) => { if (active) setReduceMotion(value); });
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    const listener = translate.addListener(({ value }) => { position.current = value; });
    return () => { active = false; subscription.remove(); translate.removeListener(listener); translate.stopAnimation(); };
  }, [translate]);

  useEffect(() => { onPeekHeight(peekHeight); }, [peekHeight, onPeekHeight]);

  function settle(full: boolean) {
    Animated.timing(translate, { toValue: full ? 0 : current.current.travel,
      duration: reduceMotion ? 0 : 240, useNativeDriver: true }).start();
  }
  const settleRef = useRef(settle);
  settleRef.current = settle;

  useEffect(() => {
    if (!expanded) scroll.current?.scrollTo({ y: 0, animated: false });
    settleRef.current(expanded);
  }, [expanded, travel, reduceMotion]);

  const responder = useMemo(() => PanResponder.create({
    onMoveShouldSetPanResponderCapture: (_, gesture) => Math.abs(gesture.dy) > 8 && Math.abs(gesture.dy) > Math.abs(gesture.dx),
    onPanResponderGrant: () => { setDragging(true); translate.stopAnimation(); dragStart.current = position.current; },
    onPanResponderMove: (_, gesture) => {
      translate.setValue(Math.max(0, Math.min(current.current.travel, dragStart.current + gesture.dy)));
    },
    onPanResponderRelease: (_, gesture) => {
      const full = Math.abs(gesture.vy) > 0.5 ? gesture.vy < 0
        : dragStart.current + gesture.dy < current.current.travel / 2;
      setDragging(false);
      current.current.onExpandedChange(full);
      // A drag back to the same position still needs to snap to its endpoint.
      settleRef.current(full);
    },
    onPanResponderTerminate: () => { setDragging(false); settleRef.current(current.current.expanded); },
  }), [translate]);

  return <Animated.View testID="route-drawer" style={[local.drawer, { height, transform: [{ translateY: translate }] }]}>
    <View {...responder.panHandlers}>
      <Pressable accessibilityRole="button"
        accessibilityLabel={expanded ? 'Collapse route drawer' : 'Expand route drawer'}
        accessibilityHint="The drawer has two positions: Peek and Full. You can also drag this handle."
        accessibilityState={{ expanded }} onPress={() => onExpandedChange(!expanded)} style={local.handle}>
        <View style={local.grip} />
        <Text style={local.handleLabel}>{expanded ? 'Show map · Peek' : 'Details and members · Full'}</Text>
      </Pressable>
    </View>
    <ScrollView ref={scroll} scrollEnabled={expanded || peekHeight === height} keyboardShouldPersistTaps="handled"
      contentContainerStyle={{ paddingBottom: bottomInset + 20 }}>
      <View onLayout={({ nativeEvent: { layout } }) => setSummaryHeight(layout.height)} style={local.summary}>{summary}</View>
      {(expanded || dragging) && <View collapsable={false} style={local.details} accessibilityElementsHidden={!expanded}
        importantForAccessibility={expanded ? 'auto' : 'no-hide-descendants'}>{children}</View>}
    </ScrollView>
  </Animated.View>;
}

const local = StyleSheet.create({
  drawer: { position: 'absolute', left: 0, right: 0, bottom: 0, backgroundColor: '#151b20',
    borderTopLeftRadius: 24, borderTopRightRadius: 24, borderColor: '#34404a', borderWidth: 1, overflow: 'hidden', elevation: 8 },
  handle: { minHeight: 48, alignItems: 'center', justifyContent: 'center', gap: 5 },
  grip: { width: 40, height: 4, borderRadius: 2, backgroundColor: '#6d7d88' },
  handleLabel: { ...styles.text, fontSize: 13, lineHeight: 18 },
  summary: { paddingHorizontal: 20, paddingBottom: 16, gap: 12 },
  details: { paddingHorizontal: 20, gap: 20 },
});
