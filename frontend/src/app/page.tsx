import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import Link from 'next/link';
import { MapPinIcon, UsersIcon, FastForwardIcon } from 'lucide-react';

export default function Home() {
    return (
        <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100">
            <div className="container mx-auto px-4 py-16">
                <div className="max-w-4xl mx-auto text-center">
                    <h1 className="text-6xl font-bold text-gray-900 mb-6">
                        Keep<span className="text-blue-600">Up</span>
                    </h1>
                    <p className="text-xl text-gray-600 mb-12">
                        Track and share your trips in real-time. Never lose your
                        group again.
                    </p>

                    <div className="grid md:grid-cols-2 gap-8 mb-12">
                        <Card>
                            <CardHeader>
                                <CardTitle className="text-2xl">
                                    Create a Trip
                                </CardTitle>
                                <CardDescription>
                                    Start tracking your journey and invite
                                    friends to join
                                </CardDescription>
                            </CardHeader>
                            <CardContent>
                                <Button asChild className="w-full">
                                    <Link href="/auth/signup">Get Started</Link>
                                </Button>
                            </CardContent>
                        </Card>

                        <Card>
                            <CardHeader>
                                <CardTitle className="text-2xl">
                                    Join a Trip
                                </CardTitle>
                                <CardDescription>
                                    Have a trip code? Join your friends and
                                    start tracking
                                </CardDescription>
                            </CardHeader>
                            <CardContent>
                                <Button
                                    variant="outline"
                                    asChild
                                    className="w-full"
                                >
                                    <Link href="/auth/login">
                                        Sign In to Join
                                    </Link>
                                </Button>
                            </CardContent>
                        </Card>
                    </div>

                    <div className="grid md:grid-cols-3 gap-6 text-left">
                        <div className="p-6">
                            <div className="w-12 h-12 bg-blue-100 rounded-lg flex items-center justify-center mb-4">
                                <MapPinIcon className="w-6 h-6 text-blue-600" />
                            </div>
                            <h3 className="text-lg font-semibold mb-2">
                                Real-time Tracking
                            </h3>
                            <p className="text-gray-600">
                                See everyone's location and path in real-time on
                                an interactive map
                            </p>
                        </div>

                        <div className="p-6">
                            <div className="w-12 h-12 bg-green-100 rounded-lg flex items-center justify-center mb-4">
                                <UsersIcon className="w-6 h-6 text-green-600" />
                            </div>
                            <h3 className="text-lg font-semibold mb-2">
                                Group Sharing
                            </h3>
                            <p className="text-gray-600">
                                Invite friends with a simple password and track
                                together
                            </p>
                        </div>

                        <div className="p-6">
                            <div className="w-12 h-12 bg-purple-100 rounded-lg flex items-center justify-center mb-4">
                                <FastForwardIcon className="w-6 h-6 text-purple-600" />
                            </div>
                            <h3 className="text-lg font-semibold mb-2">
                                Trip Replay
                            </h3>
                            <p className="text-gray-600">
                                Replay your adventures with playback controls
                                and different speeds
                            </p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
