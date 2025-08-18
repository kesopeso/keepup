'use client';

import { APIProvider, Map } from '@vis.gl/react-google-maps';
import { useEffect, useState } from 'react';

interface GoogleMapsProps {
    className?: string;
}

export default function GoogleMap({ className }: GoogleMapsProps) {
    // defaults to Ljubljana, Slovenia
    const [center, setCenter] = useState<google.maps.LatLngLiteral>({
        lat: 46.05736,
        lng: 14.50203,
    });
    const [isCenterSet, setIsCenterSet] = useState(false);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const setLoaded = () => setIsLoading(false);
    const setLoadError = (error: unknown) => setError(error as string);

    const googleMapsApiKey = process.env.NEXT_PUBLIC_GOOGLE_MAPS_API_KEY;

    useEffect(() => {
        if (!navigator || !navigator.geolocation || isCenterSet) {
            return;
        }

        navigator.geolocation.getCurrentPosition(
            (position) => {
                setIsCenterSet(true);
                setCenter({
                    lat: position.coords.latitude,
                    lng: position.coords.longitude,
                });
            },
            (error) => {
                setIsCenterSet(true);
                console.log('unable to get current position', error);
            },
            { enableHighAccuracy: true, timeout: 5000, maximumAge: 300000 }
        );
    }, [isCenterSet]);

    if (!googleMapsApiKey) {
        return (
            <div
                className={`flex items-center justify-center bg-gray-100 ${className}`}
            >
                <div className="text-center">
                    <div className="text-red-500 text-2xl mb-4">⚠️</div>
                    <p className="text-red-600">Missing Google Maps API key</p>
                </div>
            </div>
        );
    }

    return (
        <APIProvider
            apiKey={googleMapsApiKey}
            onLoad={setLoaded}
            onError={setLoadError}
        >
            {isLoading || !isCenterSet ? (
                <div
                    className={`flex items-center justify-center bg-gray-100 ${className}`}
                >
                    <div className="text-center">
                        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-4"></div>
                        <p className="text-gray-600">Loading map...</p>
                    </div>
                </div>
            ) : !!error ? (
                <div
                    className={`flex items-center justify-center bg-gray-100 ${className}`}
                >
                    <div className="text-center">
                        <div className="text-red-500 text-2xl mb-4">⚠️</div>
                        <p className="text-red-600">Error occured: {error}</p>
                    </div>
                </div>
            ) : (
                <Map
                    defaultCenter={center}
                    defaultZoom={12}
                    mapTypeId="roadmap" //googleMaps.MapTypeId.ROADMAP
                    disableDefaultUI={false}
                    zoomControl={true}
                    mapTypeControl={false}
                    scaleControl={false}
                    streetViewControl={false}
                    rotateControl={false}
                    fullscreenControl={false}
                ></Map>
            )}
        </APIProvider>
    );
}
