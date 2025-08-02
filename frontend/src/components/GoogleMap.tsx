'use client';

import { useEffect, useRef, useState } from 'react';
import { Loader } from '@googlemaps/js-api-loader';

interface GoogleMapProps {
    className?: string;
}

export default function GoogleMap({ className = '' }: GoogleMapProps) {
    const mapRef = useRef<HTMLDivElement>(null);
    const [map, setMap] = useState<google.maps.Map | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const initMap = async () => {
            const apiKey = process.env.NEXT_PUBLIC_GOOGLE_MAPS_API_KEY;
            
            if (!apiKey) {
                setError('Google Maps API key is not configured');
                setLoading(false);
                return;
            }

            // Wait a bit for the component to fully mount
            await new Promise(resolve => setTimeout(resolve, 100));

            if (!mapRef.current) {
                setError('Map container element not found');
                setLoading(false);
                return;
            }

            // Check if container has dimensions
            const rect = mapRef.current.getBoundingClientRect();
            console.log('Map container dimensions:', rect);
            
            if (rect.width === 0 || rect.height === 0) {
                setError(`Map container has no dimensions: ${rect.width}x${rect.height}`);
                setLoading(false);
                return;
            }

            try {
                const loader = new Loader({
                    apiKey: apiKey,
                    version: 'weekly',
                    libraries: ['places']
                });

                const google = await loader.load();
                
                // Default to San Francisco coordinates
                const defaultCenter = { lat: 37.7749, lng: -122.4194 };
                
                const mapInstance = new google.maps.Map(mapRef.current, {
                    center: defaultCenter,
                    zoom: 12,
                    mapTypeId: google.maps.MapTypeId.ROADMAP,
                    styles: [
                        {
                            featureType: 'poi',
                            elementType: 'labels',
                            stylers: [{ visibility: 'off' }]
                        }
                    ],
                    disableDefaultUI: false,
                    zoomControl: true,
                    mapTypeControl: false,
                    scaleControl: false,
                    streetViewControl: false,
                    rotateControl: false,
                    fullscreenControl: true
                });

                setMap(mapInstance);
                setLoading(false);

                // Try to get user's current location
                if (navigator.geolocation) {
                    navigator.geolocation.getCurrentPosition(
                        (position) => {
                            const userLocation = {
                                lat: position.coords.latitude,
                                lng: position.coords.longitude
                            };
                            mapInstance.setCenter(userLocation);
                            
                            // Add a marker for user's location
                            new google.maps.Marker({
                                position: userLocation,
                                map: mapInstance,
                                title: 'Your Location',
                                icon: {
                                    url: 'data:image/svg+xml;charset=UTF-8,' + encodeURIComponent(`
                                        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                            <circle cx="12" cy="12" r="8" fill="#3B82F6" stroke="#FFFFFF" stroke-width="2"/>
                                        </svg>
                                    `),
                                    scaledSize: new google.maps.Size(24, 24),
                                    anchor: new google.maps.Point(12, 12)
                                }
                            });
                        },
                        (error) => {
                            console.warn('Geolocation error:', error);
                            // Keep default location if geolocation fails
                        },
                        {
                            enableHighAccuracy: true,
                            timeout: 10000,
                            maximumAge: 300000 // 5 minutes
                        }
                    );
                }

            } catch (err) {
                console.error('Error loading Google Maps:', err);
                setError('Failed to load Google Maps');
                setLoading(false);
            }
        };

        initMap();
    }, []);

    if (loading) {
        return (
            <div className={`flex items-center justify-center bg-gray-100 ${className}`}>
                <div className="text-center">
                    <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-4"></div>
                    <p className="text-gray-600">Loading map...</p>
                </div>
            </div>
        );
    }

    if (error) {
        return (
            <div className={`flex items-center justify-center bg-gray-100 ${className}`}>
                <div className="text-center">
                    <div className="text-red-500 text-2xl mb-4">⚠️</div>
                    <p className="text-red-600">{error}</p>
                </div>
            </div>
        );
    }

    return (
        <div 
            ref={mapRef} 
            className={className}
            style={{ 
                width: '100%', 
                height: '100%', 
                minHeight: '400px',
                position: 'relative'
            }} 
        />
    );
}